package projects

import (
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

// Routes need work:read; each change checks its own permission in the service.
func (h *Handler) Routes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(authz.WorkRead, h.logger))
		r.Get("/projects", h.list)
		r.Post("/projects", h.create)
		r.Get("/projects/{id}", h.get)
		r.Patch("/projects/{id}", h.update)
		r.Post("/projects/{id}/{action:close|reopen|archive|unarchive}", h.lifecycle)
		r.Get("/projects/{id}/subtree", h.subtree)
		r.Put("/projects/{id}/types", h.setTypes)
		r.Put("/projects/{id}/custom-fields", h.setFields)
		r.Get("/projects/{id}/phases", h.phases)
		r.Patch("/phases/{id}", h.updatePhase)
		r.Post("/phases/{id}/gates/{gate}", h.gate)
	})
}

func (h *Handler) id(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		apperr.Write(w, h.logger, apperr.ErrNotFound)
	}
	return id, ok
}

func (h *Handler) reply(w http.ResponseWriter, status int, v any, err error) {
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, status, v)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	page, err := h.svc.List(r.Context(), r.URL.Query())
	h.reply(w, http.StatusOK, page, err)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	p, err := h.svc.Create(r.Context(), authz.FromContext(r.Context()), req)
	h.reply(w, http.StatusCreated, p, err)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	p, err := h.svc.Get(r.Context(), authz.FromContext(r.Context()), id)
	h.reply(w, http.StatusOK, p, err)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	var req updateRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	p, err := h.svc.Update(r.Context(), authz.FromContext(r.Context()), id, req)
	h.reply(w, http.StatusOK, p, err)
}

func (h *Handler) lifecycle(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	var req versionRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	p, err := h.svc.SetLifecycle(r.Context(), authz.FromContext(r.Context()), id, chi.URLParam(r, "action"), req.Version)
	h.reply(w, http.StatusOK, p, err)
}

func (h *Handler) subtree(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	nodes, err := h.svc.Subtree(r.Context(), authz.FromContext(r.Context()), id)
	h.reply(w, http.StatusOK, map[string]any{"items": nodes}, err)
}

func (h *Handler) setTypes(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	var req idsRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	p, err := h.svc.SetTypes(r.Context(), authz.FromContext(r.Context()), id, req.IDs)
	h.reply(w, http.StatusOK, p, err)
}

func (h *Handler) setFields(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	var req idsRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	p, err := h.svc.SetFields(r.Context(), authz.FromContext(r.Context()), id, req.IDs)
	h.reply(w, http.StatusOK, p, err)
}

func (h *Handler) phases(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	ps, err := h.svc.Phases(r.Context(), authz.FromContext(r.Context()), id)
	h.reply(w, http.StatusOK, map[string]any{"items": ps}, err)
}

func (h *Handler) updatePhase(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	var req phaseRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	ph, err := h.svc.UpdatePhase(r.Context(), authz.FromContext(r.Context()), id, req)
	h.reply(w, http.StatusOK, ph, err)
}

func (h *Handler) gate(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	var req gateRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	ph, err := h.svc.SetGate(r.Context(), authz.FromContext(r.Context()), id, chi.URLParam(r, "gate"), req.Passed)
	h.reply(w, http.StatusOK, ph, err)
}
