package portfolio

import (
	"log/slog"
	"net/http"
	"time"

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

// Routes: administrators, Dirección and leads, on what they see (spec 023, RF-17a).
func (h *Handler) Routes(r chi.Router) {
	r.With(auth.RequirePermission(authz.OversightView, h.logger)).Get("/portfolio", h.get)
}

// get takes today as the caller's date (YYYY-MM-DD).
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	today := time.Now().UTC()
	if v := r.URL.Query().Get("today"); v != "" {
		t, err := time.Parse(time.DateOnly, v)
		if err != nil {
			apperr.Write(w, h.logger, apperr.ErrInvalidFilter.WithDetail("field", "today"))
			return
		}
		today = t
	}
	out, err := h.svc.Get(r.Context(), authz.FromContext(r.Context()), today)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}
