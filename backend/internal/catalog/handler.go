package catalog

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

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
	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(authz.WorkRead, h.logger))
		r.Get("/catalog", h.get)
		r.Get("/work-item-types/{id}/transitions", h.transitions)
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(authz.CatalogManage, h.logger))
		r.Post("/methodologies", create(h, h.svc.CreateMethodology))
		r.Patch("/methodologies/{id}", update(h, h.svc.UpdateMethodology))
		r.Post("/methodologies/{id}/retire", retire(h, h.svc.SetMethodologyRetired, true))
		r.Post("/methodologies/{id}/restore", retire(h, h.svc.SetMethodologyRetired, false))
		r.Post("/methodologies/{id}/phases", update(h, h.svc.CreatePhase))
		r.Patch("/phase-definitions/{id}", update(h, h.svc.UpdatePhase))
		r.Post("/phase-definitions/{id}/retire", retire(h, h.svc.SetPhaseRetired, true))
		r.Post("/phase-definitions/{id}/restore", retire(h, h.svc.SetPhaseRetired, false))
		r.Post("/work-item-types", create(h, h.svc.CreateType))
		r.Patch("/work-item-types/{id}", update(h, h.svc.UpdateType))
		r.Post("/work-item-types/{id}/retire", retire(h, h.svc.SetTypeRetired, true))
		r.Post("/work-item-types/{id}/restore", retire(h, h.svc.SetTypeRetired, false))
		r.Put("/work-item-types/{id}/transitions", h.setTransitions)
		r.Post("/statuses", create(h, h.svc.CreateStatus))
		r.Patch("/statuses/{id}", update(h, h.svc.UpdateStatus))
		r.Post("/statuses/{id}/retire", retire(h, h.svc.SetStatusRetired, true))
		r.Post("/statuses/{id}/restore", retire(h, h.svc.SetStatusRetired, false))
		r.Post("/priorities", create(h, h.svc.CreatePriority))
		r.Patch("/priorities/{id}", update(h, h.svc.UpdatePriority))
		r.Post("/priorities/{id}/retire", retire(h, h.svc.SetPriorityRetired, true))
		r.Post("/priorities/{id}/restore", retire(h, h.svc.SetPriorityRetired, false))
		r.Post("/work-item-sizes", create(h, h.svc.CreateSize))
		r.Patch("/work-item-sizes/{id}", update(h, h.svc.UpdateSize))
		r.Post("/work-item-sizes/{id}/retire", retire(h, h.svc.SetSizeRetired, true))
		r.Post("/work-item-sizes/{id}/restore", retire(h, h.svc.SetSizeRetired, false))
	})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.Get(r.Context())
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, c)
}

func (h *Handler) transitions(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		apperr.Write(w, h.logger, apperr.ErrNotFound)
		return
	}
	ts, err := h.svc.Transitions(r.Context(), id)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"transitions": ts})
}

func (h *Handler) setTransitions(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		apperr.Write(w, h.logger, apperr.ErrNotFound)
		return
	}
	var req transitionsRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	ts, err := h.svc.SetTransitions(r.Context(), authz.FromContext(r.Context()), id, req.Transitions)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"transitions": ts})
}

func create[Req, Res any](h *Handler, fn func(context.Context, *authz.Principal, Req) (Res, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req Req
		if err := apperr.Decode(r, &req); err != nil {
			apperr.Write(w, h.logger, err)
			return
		}
		res, err := fn(r.Context(), authz.FromContext(r.Context()), req)
		if err != nil {
			apperr.Write(w, h.logger, err)
			return
		}
		httpx.WriteJSON(w, http.StatusCreated, res)
	}
}

func update[Req, Res any](h *Handler, fn func(context.Context, *authz.Principal, uuid.UUID, Req) (Res, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := httpx.PathUUID(r, "id")
		if !ok {
			apperr.Write(w, h.logger, apperr.ErrNotFound)
			return
		}
		var req Req
		if err := apperr.Decode(r, &req); err != nil {
			apperr.Write(w, h.logger, err)
			return
		}
		res, err := fn(r.Context(), authz.FromContext(r.Context()), id, req)
		if err != nil {
			apperr.Write(w, h.logger, err)
			return
		}
		status := http.StatusOK
		if r.Method == http.MethodPost {
			status = http.StatusCreated
		}
		httpx.WriteJSON(w, status, res)
	}
}

func retire[Res any](h *Handler, fn func(context.Context, *authz.Principal, uuid.UUID, bool) (Res, error), retired bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := httpx.PathUUID(r, "id")
		if !ok {
			apperr.Write(w, h.logger, apperr.ErrNotFound)
			return
		}
		res, err := fn(r.Context(), authz.FromContext(r.Context()), id, retired)
		if err != nil {
			apperr.Write(w, h.logger, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, res)
	}
}
