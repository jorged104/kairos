package comments

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"openscum/internal/apperr"
	"openscum/internal/auth"
	"openscum/internal/authz"
	"openscum/internal/pages"
	"openscum/pkg/httpx"
)

type Handler struct {
	svc    *Service
	logger *slog.Logger
}

func NewHandler(svc *Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

// Routes need work:read; each change checks work:comment and the comment's
// own rules (RF-23 to RF-35).
func (h *Handler) Routes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(authz.WorkRead, h.logger))
		r.Get("/work-items/{id}/comments", h.list)
		r.Post("/work-items/{id}/comments", h.create)
		r.Post("/work-items/{id}/comment-files", h.upload)
		r.Patch("/comments/{id}", h.update)
		r.Delete("/comments/{id}", h.with(h.svc.Delete))
		r.Post("/comments/{id}/resolve", h.with(func(ctx context.Context, p *authz.Principal, id uuid.UUID) (Result, error) {
			return h.svc.SetResolved(ctx, p, id, true)
		}))
		r.Post("/comments/{id}/reopen", h.with(func(ctx context.Context, p *authz.Principal, id uuid.UUID) (Result, error) {
			return h.svc.SetResolved(ctx, p, id, false)
		}))
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		apperr.Write(w, h.logger, apperr.ErrNotFound)
		return
	}
	out, err := h.svc.List(r.Context(), authz.FromContext(r.Context()), id)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		apperr.Write(w, h.logger, apperr.ErrNotFound)
		return
	}
	var req createRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	out, err := h.svc.Create(r.Context(), authz.FromContext(r.Context()), id, req)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, out)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		apperr.Write(w, h.logger, apperr.ErrNotFound)
		return
	}
	var req updateRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	out, err := h.svc.Update(r.Context(), authz.FromContext(r.Context()), id, req)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) with(fn func(context.Context, *authz.Principal, uuid.UUID) (Result, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := httpx.PathUUID(r, "id")
		if !ok {
			apperr.Write(w, h.logger, apperr.ErrNotFound)
			return
		}
		out, err := fn(r.Context(), authz.FromContext(r.Context()), id)
		if err != nil {
			apperr.Write(w, h.logger, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, out)
	}
}

// upload receives a file for the comments of a work item (RF-26).
func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		apperr.Write(w, h.logger, apperr.ErrNotFound)
		return
	}
	name, data, err := pages.ReadUpload(w, r)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	f, err := h.svc.Upload(r.Context(), authz.FromContext(r.Context()), id, name, data)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, f)
}
