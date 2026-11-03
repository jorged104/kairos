package workitems

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

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
		r.Get("/projects/{id}/work-items", h.list)
		r.Post("/projects/{id}/work-items", h.create)
		r.Get("/projects/{id}/tree", h.tree)
		r.Get("/work-items/by-key/{key}", h.byKey)
		r.Get("/work-items/{id}", h.get)
		r.Patch("/work-items/{id}", h.update)
		r.Delete("/work-items/{id}", h.delete)
		r.Get("/work-items/{id}/ancestors", h.ancestors)
		r.Get("/work-items/{id}/descendants", h.descendants)
		r.Get("/work-items/{id}/metrics", h.metrics)
		r.Get("/work-items/{id}/history", h.history)
		r.Post("/work-items/{id}/dependencies", h.addDependency)
		r.Delete("/dependencies/{id}", h.removeDependency)
	})
}

func (h *Handler) id(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		apperr.Write(w, h.logger, apperr.ErrNotFound)
	}
	return id, ok
}

// metrics takes today as the caller's date (YYYY-MM-DD); without it, the
// server's date in UTC (spec 007).
func (h *Handler) metrics(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	today := time.Now().UTC()
	if v := r.URL.Query().Get("today"); v != "" {
		t, err := time.Parse(time.DateOnly, v)
		if err != nil {
			apperr.Write(w, h.logger, apperr.ErrInvalidFilter.WithDetail("field", "today"))
			return
		}
		today = t
	}
	out, err := h.svc.Metrics(r.Context(), authz.FromContext(r.Context()), id, today)
	h.reply(w, http.StatusOK, out, err)
}

// history pages by the cursor of the previous answer (spec 007, RF-52).
func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	var before *int64
	if v := r.URL.Query().Get("before"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			apperr.Write(w, h.logger, apperr.ErrInvalidFilter.WithDetail("field", "before"))
			return
		}
		before = &n
	}
	out, err := h.svc.History(r.Context(), authz.FromContext(r.Context()), id, before)
	h.reply(w, http.StatusOK, out, err)
}

func (h *Handler) reply(w http.ResponseWriter, status int, v any, err error) {
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, status, v)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	page, err := h.svc.List(r.Context(), authz.FromContext(r.Context()), id, r.URL.Query())
	h.reply(w, http.StatusOK, page, err)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	var req createRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	wi, err := h.svc.Create(r.Context(), authz.FromContext(r.Context()), id, req)
	h.reply(w, http.StatusCreated, wi, err)
}

func (h *Handler) tree(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	nodes, err := h.svc.Tree(r.Context(), authz.FromContext(r.Context()), id, r.URL.Query())
	h.reply(w, http.StatusOK, map[string]any{"items": nodes}, err)
}

func (h *Handler) byKey(w http.ResponseWriter, r *http.Request) {
	wi, err := h.svc.GetByKey(r.Context(), authz.FromContext(r.Context()), chi.URLParam(r, "key"))
	h.reply(w, http.StatusOK, wi, err)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	wi, err := h.svc.Get(r.Context(), authz.FromContext(r.Context()), id)
	h.reply(w, http.StatusOK, wi, err)
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
	wi, err := h.svc.Update(r.Context(), authz.FromContext(r.Context()), id, req)
	h.reply(w, http.StatusOK, wi, err)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), authz.FromContext(r.Context()), id); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ancestors(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	refs, err := h.svc.Ancestors(r.Context(), authz.FromContext(r.Context()), id)
	h.reply(w, http.StatusOK, map[string]any{"items": refs}, err)
}

func (h *Handler) descendants(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	nodes, err := h.svc.Descendants(r.Context(), authz.FromContext(r.Context()), id)
	h.reply(w, http.StatusOK, map[string]any{"items": nodes}, err)
}

func (h *Handler) addDependency(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	var req dependencyRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	deps, err := h.svc.AddDependency(r.Context(), authz.FromContext(r.Context()), id, req)
	h.reply(w, http.StatusCreated, map[string]any{"items": deps}, err)
}

func (h *Handler) removeDependency(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	if err := h.svc.RemoveDependency(r.Context(), authz.FromContext(r.Context()), id); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
