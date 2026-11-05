package meetings

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

// Routes need work:read; each change checks its own permission.
func (h *Handler) Routes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(authz.WorkRead, h.logger))
		r.Get("/projects/{id}/meetings", h.list)
		r.Post("/projects/{id}/meetings", h.create)
		r.Get("/meetings/{id}", h.get)
		r.Patch("/meetings/{id}", h.update)
		r.Delete("/meetings/{id}", h.delete)
		r.Get("/meeting-templates", h.choose)
		r.Get("/meeting-templates/global", h.global)
		r.Post("/meeting-templates", h.createTemplate)
		r.Patch("/meeting-templates/{id}", h.updateTemplate)
		r.Post("/meeting-templates/{id}/{action:retire|restore}", h.retireTemplate)
		r.Delete("/meeting-templates/{id}", h.deleteTemplate)
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
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	page, ok := httpx.ParsePage(q, PageSize, PageSize)
	if !ok {
		apperr.Write(w, h.logger, apperr.ErrInvalidFilter.WithDetail("field", "page"))
		return
	}
	upcoming := true
	switch q.Get("when") {
	case "", "upcoming":
	case "past":
		upcoming = false
	default:
		apperr.Write(w, h.logger, apperr.ErrInvalidFilter.WithDetail("field", "when"))
		return
	}
	out, err := h.svc.List(r.Context(), authz.FromContext(r.Context()), id, upcoming, page.Number, page.Size)
	h.reply(w, http.StatusOK, out, err)
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
	out, err := h.svc.Create(r.Context(), authz.FromContext(r.Context()), id, req)
	h.reply(w, http.StatusCreated, out, err)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	out, err := h.svc.Get(r.Context(), authz.FromContext(r.Context()), id)
	h.reply(w, http.StatusOK, out, err)
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
	out, err := h.svc.Update(r.Context(), authz.FromContext(r.Context()), id, req)
	h.reply(w, http.StatusOK, out, err)
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

func (h *Handler) choose(w http.ResponseWriter, r *http.Request) {
	project, err := uuid.Parse(r.URL.Query().Get("project_id"))
	if err != nil {
		apperr.Write(w, h.logger, apperr.ErrInvalidFilter.WithDetail("field", "project_id"))
		return
	}
	out, err := h.svc.Choose(r.Context(), authz.FromContext(r.Context()), project)
	h.reply(w, http.StatusOK, map[string]any{"items": out}, err)
}

func (h *Handler) global(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Global(r.Context(), authz.FromContext(r.Context()))
	h.reply(w, http.StatusOK, map[string]any{"items": out}, err)
}

func (h *Handler) createTemplate(w http.ResponseWriter, r *http.Request) {
	var req templateRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	out, err := h.svc.CreateTemplate(r.Context(), authz.FromContext(r.Context()), req)
	h.reply(w, http.StatusCreated, out, err)
}

func (h *Handler) updateTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	var req templateRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	out, err := h.svc.UpdateTemplate(r.Context(), authz.FromContext(r.Context()), id, req)
	h.reply(w, http.StatusOK, out, err)
}

func (h *Handler) retireTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	out, err := h.svc.SetTemplateRetired(r.Context(), authz.FromContext(r.Context()), id, chi.URLParam(r, "action") == "retire")
	h.reply(w, http.StatusOK, out, err)
}

func (h *Handler) deleteTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteTemplate(r.Context(), authz.FromContext(r.Context()), id); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
