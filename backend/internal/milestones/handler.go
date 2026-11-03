package milestones

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

// Routes need work:read; changes check the project manager in the service.
func (h *Handler) Routes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(authz.WorkRead, h.logger))
		r.Get("/projects/{id}/milestones", h.list)
		r.Post("/projects/{id}/milestones", h.create)
		r.Get("/milestones/{id}", h.get)
		r.Patch("/milestones/{id}", h.update)
		r.Delete("/milestones/{id}", h.delete)
		r.Post("/milestones/{id}/{action:complete|reopen|cancel}", h.status)
		r.Get("/milestones/{id}/work-items", h.workItems)
		r.Post("/milestones/{id}/work-items", h.link)
		r.Post("/milestones/{id}/work-items/unlink", h.unlinkMany)
		r.Delete("/milestones/{id}/work-items/{work_item_id}", h.unlink)
		r.Get("/work-items/{id}/milestones", h.ofWorkItem)
	})
}

func (h *Handler) uuid(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, ok := httpx.PathUUID(r, name)
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
	id, ok := h.uuid(w, r, "id")
	if !ok {
		return
	}
	ms, err := h.svc.List(r.Context(), authz.FromContext(r.Context()), id)
	h.reply(w, http.StatusOK, map[string]any{"items": ms}, err)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	id, ok := h.uuid(w, r, "id")
	if !ok {
		return
	}
	var req milestoneRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	m, err := h.svc.Create(r.Context(), authz.FromContext(r.Context()), id, req)
	h.reply(w, http.StatusCreated, m, err)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := h.uuid(w, r, "id")
	if !ok {
		return
	}
	m, err := h.svc.Get(r.Context(), authz.FromContext(r.Context()), id)
	h.reply(w, http.StatusOK, m, err)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := h.uuid(w, r, "id")
	if !ok {
		return
	}
	var req milestoneRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	m, err := h.svc.Update(r.Context(), authz.FromContext(r.Context()), id, req)
	h.reply(w, http.StatusOK, m, err)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := h.uuid(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), authz.FromContext(r.Context()), id); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	id, ok := h.uuid(w, r, "id")
	if !ok {
		return
	}
	var req statusRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	m, err := h.svc.SetStatus(r.Context(), authz.FromContext(r.Context()), id, chi.URLParam(r, "action"), req)
	h.reply(w, http.StatusOK, m, err)
}

func (h *Handler) workItems(w http.ResponseWriter, r *http.Request) {
	id, ok := h.uuid(w, r, "id")
	if !ok {
		return
	}
	items, err := h.svc.WorkItems(r.Context(), authz.FromContext(r.Context()), id)
	h.reply(w, http.StatusOK, map[string]any{"items": items}, err)
}

func (h *Handler) link(w http.ResponseWriter, r *http.Request) {
	id, ok := h.uuid(w, r, "id")
	if !ok {
		return
	}
	var req linkRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	n, err := h.svc.Link(r.Context(), authz.FromContext(r.Context()), id, req.ids(), true)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	// One work item answers as before; several, how many were linked.
	if req.WorkItemIDs == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	h.reply(w, http.StatusOK, map[string]int{"linked": n}, nil)
}

// unlinkMany unlinks several work items at once (RF-168).
func (h *Handler) unlinkMany(w http.ResponseWriter, r *http.Request) {
	id, ok := h.uuid(w, r, "id")
	if !ok {
		return
	}
	var req linkRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	n, err := h.svc.Link(r.Context(), authz.FromContext(r.Context()), id, req.ids(), false)
	h.reply(w, http.StatusOK, map[string]int{"unlinked": n}, err)
}

func (h *Handler) unlink(w http.ResponseWriter, r *http.Request) {
	id, ok := h.uuid(w, r, "id")
	if !ok {
		return
	}
	wid, ok := h.uuid(w, r, "work_item_id")
	if !ok {
		return
	}
	n, err := h.svc.Link(r.Context(), authz.FromContext(r.Context()), id, []uuid.UUID{wid}, false)
	if err == nil && n == 0 {
		err = apperr.ErrNotFound // one that was not linked, as before
	}
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ofWorkItem(w http.ResponseWriter, r *http.Request) {
	id, ok := h.uuid(w, r, "id")
	if !ok {
		return
	}
	ms, err := h.svc.OfWorkItem(r.Context(), authz.FromContext(r.Context()), id)
	h.reply(w, http.StatusOK, map[string]any{"items": ms}, err)
}
