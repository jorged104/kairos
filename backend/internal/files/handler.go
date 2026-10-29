package files

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

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

// Routes serve the files; uploading belongs to the pages package, which knows
// who may change each owner.
func (h *Handler) Routes(r chi.Router) {
	r.With(auth.RequirePermission(authz.WorkRead, h.logger)).Get("/files/{id}", h.get)
}

// get serves a file so the browser never runs it in Kairos's origin (RF-24):
// images inline, everything else as a download, with nosniff and a sandbox.
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		apperr.Write(w, h.logger, apperr.ErrNotFound)
		return
	}
	f, body, err := h.svc.Open(r.Context(), authz.FromContext(r.Context()), id)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	defer body.Close()
	disposition := "attachment"
	if f.Kind == "image" {
		disposition = "inline"
	}
	hd := w.Header()
	hd.Set("Content-Type", f.ContentType)
	hd.Set("Content-Length", strconv.FormatInt(f.SizeBytes, 10))
	hd.Set("Content-Disposition", disposition+"; filename*=UTF-8''"+url.PathEscape(f.Name))
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	hd.Set("Cache-Control", "private, max-age=3600")
	if _, err := io.Copy(w, body); err != nil {
		h.logger.Warn("enviar archivo", slog.String("error", err.Error()))
	}
}
