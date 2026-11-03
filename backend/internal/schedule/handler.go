package schedule

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"openscum/internal/apperr"
	"openscum/internal/auth"
	"openscum/internal/authz"
	"openscum/pkg/httpx"
)

type Handler struct {
	svc    *Service
	logger *slog.Logger
}

func NewHandler(svc *Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

// Routes: everyone who reads work sees the settings; changing them is for
// whoever manages the catalog, the system administrator (RF-36, RF-42).
func (h *Handler) Routes(r chi.Router) {
	r.With(auth.RequirePermission(authz.WorkRead, h.logger)).Get("/schedule-settings", h.get)
	r.With(auth.RequirePermission(authz.CatalogManage, h.logger)).Put("/schedule-settings", h.update)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	s, err := h.svc.Get(r.Context())
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, s)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var req settingsRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	s, err := h.svc.Update(r.Context(), authz.FromContext(r.Context()), req)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, s)
}
