package auth

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"openscum/internal/apperr"
	"openscum/internal/authz"
	"openscum/internal/config"
	"openscum/internal/db"
	"openscum/pkg/httpx"
)

type Handler struct {
	cfg      config.AuthConfig
	svc      *Service
	store    *db.Store
	sessions *Sessions
	logger   *slog.Logger
	// endSessionURL returns the IdP logout URL, or "" if there is none (RF-52, RF-53).
	endSessionURL func(ctx context.Context) string
}

func NewHandler(cfg config.AuthConfig, svc *Service, store *db.Store, sessions *Sessions, logger *slog.Logger) *Handler {
	return &Handler{cfg: cfg, svc: svc, store: store, sessions: sessions, logger: logger}
}

// SetEndSessionURL plugs in the OIDC logout redirect.
func (h *Handler) SetEndSessionURL(f func(ctx context.Context) string) { h.endSessionURL = f }

// PublicRoutes are reachable without a session.
func (h *Handler) PublicRoutes(r chi.Router) {
	r.Get("/auth/config", h.Config)
	r.Post("/auth/login", h.Login)
	r.Post("/auth/register", h.Register)
	r.Post("/auth/logout", h.Logout)
}

// Routes require a session.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/me", h.Me)
	r.Put("/me/password", h.ChangePassword)
}

func (h *Handler) Config(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, ConfigResponse{
		LocalEnabled:        h.cfg.LocalEnabled(),
		OIDCEnabled:         h.cfg.OIDCEnabled(),
		RegistrationEnabled: h.cfg.LocalEnabled() && h.cfg.RegistrationEnabled,
	})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	id, err := h.svc.Login(r.Context(), req.Email, req.Password, clientIP(r, h.cfg.TrustProxy))
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	h.openSession(w, r, id, MethodLocal)
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	id, err := h.svc.Register(r.Context(), req)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	h.openSession(w, r, id, MethodLocal)
}

// openSession sets the cookie and answers with the user's own view.
func (h *Handler) openSession(w http.ResponseWriter, r *http.Request, id uuid.UUID, method string) {
	if err := h.sessions.Issue(w, id, method); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	p, err := h.svc.loader.Load(r.Context(), id)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	me, err := loadMe(r.Context(), h.store, p, method)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, me)
}

// Logout clears the session even without a valid one, and tells the
// frontend where to go next.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	method := h.sessions.Method(r)
	h.sessions.Clear(w)
	redirect := "/login"
	if method == MethodOIDC && h.endSessionURL != nil {
		if u := h.endSessionURL(r.Context()); u != "" {
			redirect = u
		}
	}
	httpx.WriteJSON(w, http.StatusOK, logoutResponse{RedirectURL: redirect})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	me, err := loadMe(r.Context(), h.store, authz.FromContext(r.Context()), h.sessions.Method(r))
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, me)
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	if err := h.svc.ChangeOwnPassword(r.Context(), authz.FromContext(r.Context()), req.CurrentPassword, req.NewPassword); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
