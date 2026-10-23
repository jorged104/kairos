package objectives

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

// Routes need work:read; changes check the objective permissions in the service.
func (h *Handler) Routes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(authz.WorkRead, h.logger))
		r.Get("/objectives", h.list)
		r.Post("/objectives", h.create)
		r.Get("/objectives/{id}", h.get)
		r.Patch("/objectives/{id}", h.update)
		r.Post("/objectives/{id}/{action:activate|close|cancel|reopen}", h.status)
		r.Post("/objectives/{id}/key-results", h.createKeyResult)
		r.Patch("/objectives/{id}/key-results/{kr}", h.updateKeyResult)
		r.Delete("/objectives/{id}/key-results/{kr}", h.deleteKeyResult)
		r.Post("/objectives/{id}/projects", h.link(h.svc.LinkProject, true))
		r.Delete("/objectives/{id}/projects/{other}", h.link(h.svc.LinkProject, false))
		r.Post("/objectives/{id}/milestones", h.link(h.svc.LinkMilestone, true))
		r.Delete("/objectives/{id}/milestones/{other}", h.link(h.svc.LinkMilestone, false))
		r.Post("/objectives/{id}/key-results/{kr}/work-items", h.krLink(h.svc.LinkKeyResultWorkItem, true))
		r.Delete("/objectives/{id}/key-results/{kr}/work-items/{other}", h.krLink(h.svc.LinkKeyResultWorkItem, false))
		r.Post("/objectives/{id}/key-results/{kr}/milestones", h.krLink(h.svc.LinkKeyResultMilestone, true))
		r.Delete("/objectives/{id}/key-results/{kr}/milestones/{other}", h.krLink(h.svc.LinkKeyResultMilestone, false))
	})
}

func (h *Handler) ids(w http.ResponseWriter, r *http.Request, names ...string) ([]uuid.UUID, bool) {
	out := make([]uuid.UUID, 0, len(names))
	for _, n := range names {
		id, ok := httpx.PathUUID(r, n)
		if !ok {
			apperr.Write(w, h.logger, apperr.ErrNotFound)
			return nil, false
		}
		out = append(out, id)
	}
	return out, true
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
	o, err := h.svc.Create(r.Context(), authz.FromContext(r.Context()), req)
	h.reply(w, http.StatusCreated, o, err)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	ids, ok := h.ids(w, r, "id")
	if !ok {
		return
	}
	o, err := h.svc.Get(r.Context(), authz.FromContext(r.Context()), ids[0])
	h.reply(w, http.StatusOK, o, err)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	ids, ok := h.ids(w, r, "id")
	if !ok {
		return
	}
	var req updateRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	o, err := h.svc.Update(r.Context(), authz.FromContext(r.Context()), ids[0], req)
	h.reply(w, http.StatusOK, o, err)
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	ids, ok := h.ids(w, r, "id")
	if !ok {
		return
	}
	var req versionRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	o, err := h.svc.SetStatus(r.Context(), authz.FromContext(r.Context()), ids[0], chi.URLParam(r, "action"), req.Version)
	h.reply(w, http.StatusOK, o, err)
}

func (h *Handler) createKeyResult(w http.ResponseWriter, r *http.Request) {
	ids, ok := h.ids(w, r, "id")
	if !ok {
		return
	}
	var req keyResultRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	o, err := h.svc.CreateKeyResult(r.Context(), authz.FromContext(r.Context()), ids[0], req)
	h.reply(w, http.StatusCreated, o, err)
}

func (h *Handler) updateKeyResult(w http.ResponseWriter, r *http.Request) {
	ids, ok := h.ids(w, r, "id", "kr")
	if !ok {
		return
	}
	var req keyResultRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	o, err := h.svc.UpdateKeyResult(r.Context(), authz.FromContext(r.Context()), ids[0], ids[1], req)
	h.reply(w, http.StatusOK, o, err)
}

func (h *Handler) deleteKeyResult(w http.ResponseWriter, r *http.Request) {
	ids, ok := h.ids(w, r, "id", "kr")
	if !ok {
		return
	}
	o, err := h.svc.DeleteKeyResult(r.Context(), authz.FromContext(r.Context()), ids[0], ids[1])
	h.reply(w, http.StatusOK, o, err)
}

type linkFunc func(ctx context.Context, p *authz.Principal, id, other uuid.UUID, linked bool) (Objective, error)

func (h *Handler) link(fn linkFunc, linked bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ids, ok := h.ids(w, r, "id")
		if !ok {
			return
		}
		other, ok := h.other(w, r, linked)
		if !ok {
			return
		}
		o, err := fn(r.Context(), authz.FromContext(r.Context()), ids[0], other, linked)
		h.reply(w, http.StatusOK, o, err)
	}
}

type krLinkFunc func(ctx context.Context, p *authz.Principal, id, kr, other uuid.UUID, linked bool) (Objective, error)

func (h *Handler) krLink(fn krLinkFunc, linked bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ids, ok := h.ids(w, r, "id", "kr")
		if !ok {
			return
		}
		other, ok := h.other(w, r, linked)
		if !ok {
			return
		}
		o, err := fn(r.Context(), authz.FromContext(r.Context()), ids[0], ids[1], other, linked)
		h.reply(w, http.StatusOK, o, err)
	}
}

// other reads the linked element: from the body when linking, from the path
// when unlinking.
func (h *Handler) other(w http.ResponseWriter, r *http.Request, linked bool) (uuid.UUID, bool) {
	if !linked {
		ids, ok := h.ids(w, r, "other")
		if !ok {
			return uuid.Nil, false
		}
		return ids[0], true
	}
	var req linkRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return uuid.Nil, false
	}
	return req.ID, true
}
