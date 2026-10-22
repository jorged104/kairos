package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"openscum/pkg/httpx"
)

// NewRouter builds the HTTP router with the shared middlewares.
func NewRouter(logger *slog.Logger) chi.Router {
	r := chi.NewRouter()
	r.Use(requestLogger(logger), recoverer(logger))

	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "Recurso no encontrado")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Método no permitido")
	})

	return r
}
