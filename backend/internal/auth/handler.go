package auth

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"openscum/internal/apperr"
	"openscum/internal/authz"
	"openscum/internal/config"
	"openscum/internal/db"
	"openscum/pkg/httpx"
)

type Handler struct {
	cfg      config.AuthConfig
	store    *db.Store
	sessions *Sessions
	logger   *slog.Logger
}

func NewHandler(cfg config.AuthConfig, store *db.Store, sessions *Sessions, logger *slog.Logger) *Handler {
	return &Handler{cfg: cfg, store: store, sessions: sessions, logger: logger}
}

// PublicRoutes are reachable without a session.
func (h *Handler) PublicRoutes(r chi.Router) {
	r.Get("/auth/config", h.Config)
}

// Routes require a session.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/me", h.Me)
}

func (h *Handler) Config(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, ConfigResponse{
		LocalEnabled:        h.cfg.LocalEnabled(),
		OIDCEnabled:         h.cfg.OIDCEnabled(),
		RegistrationEnabled: h.cfg.LocalEnabled() && h.cfg.RegistrationEnabled,
	})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	me, err := loadMe(r.Context(), h.store, authz.FromContext(r.Context()), h.sessions.Method(r))
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, me)
}
