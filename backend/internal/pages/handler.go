package pages

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

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
			r.Post(path+"/files", h.upload(kind))
		}
	})
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
