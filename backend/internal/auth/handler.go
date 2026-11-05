package auth

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"openscum/internal/apperr"
	"openscum/internal/authz"
	"openscum/internal/config"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
	"openscum/pkg/httpx"
)

type Handler struct {
	cfg      func() config.AuthConfig
	svc      *Service
	store    *db.Store
	sessions *Sessions
	logger   *slog.Logger
	// endSessionURL returns the IdP logout URL, or "" if there is none (RF-52, RF-53).
	endSessionURL func(ctx context.Context) string
	theme         func() any
}

func NewHandler(cfg func() config.AuthConfig, svc *Service, store *db.Store, sessions *Sessions, logger *slog.Logger) *Handler {
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
	r.Put("/me/preferences", h.SetPreferences)
}

// SetTheme gives the login screen the appearance of the system (spec 015).
func (h *Handler) SetTheme(f func() any) { h.theme = f }

func (h *Handler) Config(w http.ResponseWriter, _ *http.Request) {
	cfg := h.cfg()
	out := ConfigResponse{
		LocalEnabled:        cfg.LocalEnabled(),
		OIDCEnabled:         cfg.OIDCEnabled(),
		RegistrationEnabled: cfg.LocalEnabled() && cfg.RegistrationEnabled,
		EmergencyEnabled:    !cfg.LocalEnabled() && cfg.BootstrapAdminEmail != "",
	}
	if h.theme != nil {
		out.Theme = h.theme()
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// SetPreferences keeps the caller's light or dark mode on every device (spec 015, RF-48 to RF-50).
func (h *Handler) SetPreferences(w http.ResponseWriter, r *http.Request) {
	var req preferencesRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	p := authz.FromContext(r.Context())
	if err := h.store.SetThemeMode(r.Context(), sqlc.SetThemeModeParams{UserID: p.UserID, ThemeMode: req.ThemeMode}); err != nil {
		apperr.Write(w, h.logger, fmt.Errorf("guardar preferencias: %w", err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	id, err := h.svc.Login(r.Context(), req.Email, req.Password, clientIP(r, h.cfg().TrustProxy))
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
