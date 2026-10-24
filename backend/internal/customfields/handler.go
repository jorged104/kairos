package customfields

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

// Routes require a session; reads need work:read and changes catalog:manage.
func (h *Handler) Routes(r chi.Router) {
	r.With(auth.RequirePermission(authz.WorkRead, h.logger)).Get("/custom-fields", h.list)
	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(authz.CatalogManage, h.logger))
		r.Post("/custom-fields", h.create)
		r.Patch("/custom-fields/{id}", h.update)
		r.Post("/custom-fields/{id}/retire", h.retire(true))
		r.Post("/custom-fields/{id}/restore", h.retire(false))
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	fs, err := h.svc.List(r.Context())
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": fs})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req fieldRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	f, err := h.svc.Create(r.Context(), authz.FromContext(r.Context()), req)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, f)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		apperr.Write(w, h.logger, apperr.ErrNotFound)
		return
	}
	var req fieldRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	f, err := h.svc.Update(r.Context(), authz.FromContext(r.Context()), id, req)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, f)
}

func (h *Handler) retire(retired bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := httpx.PathUUID(r, "id")
		if !ok {
			apperr.Write(w, h.logger, apperr.ErrNotFound)
			return
		}
		f, err := h.svc.SetRetired(r.Context(), authz.FromContext(r.Context()), id, retired)
		if err != nil {
			apperr.Write(w, h.logger, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, f)
	}
}
