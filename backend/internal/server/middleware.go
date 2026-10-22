package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"openscum/pkg/httpx"
)

// requestLogger logs every request with method, path, status and duration.
func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}
			logger.Info("request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", status),
				slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			)
		})
	}
}

// recoverer turns a panic into a 500 response in the common error format.
func recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic en el manejador", slog.Any("panic", rec))
					httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "Error interno del servidor")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
