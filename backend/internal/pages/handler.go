package pages

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"

	"openscum/internal/apperr"
	"openscum/internal/auth"
	"openscum/internal/authz"
	"openscum/internal/files"
	"openscum/pkg/httpx"
)

type Handler struct {
	svc    *Service
	logger *slog.Logger
}

func NewHandler(svc *Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

var routes = map[string]string{
	KindWorkItem:  "/work-items/{id}",
	KindMilestone: "/milestones/{id}",
	KindProject:   "/projects/{id}",
}

// Routes need work:read; each change checks the owner's own permission.
func (h *Handler) Routes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(authz.WorkRead, h.logger))
		for kind, path := range routes {
			r.Put(path+"/page", h.save(kind))
			r.Post(path+"/files", h.upload(kind))
		}
		r.Get("/files/{id}", h.file)
	})
}

// save is the autosave of a page (RF-31 to RF-36, RF-59).
func (h *Handler) save(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := httpx.PathUUID(r, "id")
		if !ok {
			apperr.Write(w, h.logger, apperr.ErrNotFound)
			return
		}
		var req saveRequest
		if err := apperr.Decode(r, &req); err != nil {
			apperr.Write(w, h.logger, err)
			return
		}
		out, err := h.svc.Save(r.Context(), authz.FromContext(r.Context()), kind, id, req)
		if err != nil {
			apperr.Write(w, h.logger, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, out)
	}
}

// file serves a file so the browser never runs it in Kairos's origin (RF-24):
// images inline, everything else as a download, with nosniff and a sandbox.
func (h *Handler) file(w http.ResponseWriter, r *http.Request) {
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

// multipartSlack is room for the multipart headers around the file.
const multipartSlack = 64 << 10

// upload receives one file in the multipart field "file" (RF-14 to RF-16).
func (h *Handler) upload(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := httpx.PathUUID(r, "id")
		if !ok {
			apperr.Write(w, h.logger, apperr.ErrNotFound)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, files.MaxBytes+multipartSlack)
		name, data, err := readFile(r)
		if err != nil {
			apperr.Write(w, h.logger, err)
			return
		}
		f, err := h.svc.Upload(r.Context(), authz.FromContext(r.Context()), kind, id, name, data)
		if err != nil {
			apperr.Write(w, h.logger, err)
			return
		}
		httpx.WriteJSON(w, http.StatusCreated, f)
	}
}

func readFile(r *http.Request) (string, []byte, error) {
	mr, err := r.MultipartReader()
	if err != nil {
		return "", nil, apperr.ErrValidation.WithDetail("field", "file")
	}
	for {
		part, err := mr.NextPart()
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return "", nil, files.ErrFileTooLarge
		}
		if err != nil {
			return "", nil, apperr.ErrValidation.WithDetail("field", "file")
		}
		if part.FormName() != "file" {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(part, files.MaxBytes+1))
		if errors.As(err, &tooBig) || len(data) > files.MaxBytes {
			return "", nil, files.ErrFileTooLarge
		}
		if err != nil {
			return "", nil, apperr.ErrValidation.WithDetail("field", "file")
		}
		return part.FileName(), data, nil
	}
}
