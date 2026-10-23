package attributes

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

// Routes require a session; changes also require attributes:manage.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/attributes", h.List)
	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(authz.AttributesManage, h.logger))
		r.Post("/attributes", h.Create)
		r.Patch("/attributes/{id}", h.Update)
		r.Post("/attributes/{id}/retire", h.retire(true))
		r.Post("/attributes/{id}/restore", h.retire(false))
	})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.List(r.Context(), authz.FromContext(r.Context()))
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	a, err := h.svc.Create(r.Context(), authz.FromContext(r.Context()), req)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, a)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
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
	a, err := h.svc.Update(r.Context(), authz.FromContext(r.Context()), id, req)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a)
}

func (h *Handler) retire(retired bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := httpx.PathUUID(r, "id")
		if !ok {
			apperr.Write(w, h.logger, apperr.ErrNotFound)
			return
		}
		a, err := h.svc.SetRetired(r.Context(), authz.FromContext(r.Context()), id, retired)
		if err != nil {
			apperr.Write(w, h.logger, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, a)
	}
}
