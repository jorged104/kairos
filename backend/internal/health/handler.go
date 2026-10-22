package health

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"openscum/pkg/httpx"
)

type Handler struct {
	svc    *Service
	logger *slog.Logger
}

func NewHandler(svc *Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

func (h *Handler) Routes(r chi.Router) {
	r.Get("/health/live", h.Live)
	r.Get("/health/ready", h.Ready)
}

// Live answers while the process runs; it never touches the database.
func (h *Handler) Live(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, statusResponse{Status: "ok"})
}

// Ready answers 200 only if the database responds in time.
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DatabaseReady(r.Context()); err != nil {
		h.logger.Warn("la base de datos no está disponible", slog.String("error", err.Error()))
		httpx.WriteError(w, http.StatusServiceUnavailable, "database_unavailable", "La base de datos no está disponible")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, statusResponse{Status: "ok", Database: "up"})
}
