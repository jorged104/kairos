package reports

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
	gen    *Generator
	logger *slog.Logger
}

func NewHandler(svc *Service, gen *Generator, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, gen: gen, logger: logger}
}

// Routes need work:read; each change checks its own permission.
func (h *Handler) Routes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(authz.WorkRead, h.logger))
		r.Get("/projects/{id}/reports", h.list)
		r.Post("/projects/{id}/reports", h.create)
		r.Post("/projects/{id}/reports/generate", h.generate)
		r.Get("/reports/{id}", h.get)
		r.Put("/reports/{id}", h.update)
		r.Delete("/reports/{id}", h.delete)
		r.Post("/reports/preview", h.preview)
		r.Get("/report-templates", h.activeTemplates)
		r.Get("/system/report-templates", h.allTemplates)
		r.Post("/system/report-templates", h.createTemplate)
		r.Put("/system/report-templates/{id}", h.updateTemplate)
		r.Post("/system/report-templates/{id}/archive", h.archiveTemplate)
	})
}

// AgentToolPath is where the agent sends its queries, outside the session
// routes: the token of the generation is its credential (ADR 0004).
const AgentToolPath = "/reports/agent/tools/{tool}"

func (h *Handler) ServeTool(w http.ResponseWriter, r *http.Request) {
	h.gen.ServeTool(w, r, chi.URLParam(r, "tool"))
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
	out, err := h.svc.List(r.Context(), authz.FromContext(r.Context()), id)
	h.reply(w, http.StatusOK, out, err)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	out, err := h.svc.Get(r.Context(), authz.FromContext(r.Context()), id)
	h.reply(w, http.StatusOK, out, err)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	var req saveRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	out, err := h.svc.Create(r.Context(), authz.FromContext(r.Context()), id, req)
	h.reply(w, http.StatusCreated, out, err)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	var req saveRequest
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

type previewRequest struct {
	Content string `json:"content" validate:"max=200000"`
}

// preview renders a draft as it will look saved (RF-31, RF-32).
func (h *Handler) preview(w http.ResponseWriter, r *http.Request) {
	var req previewRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	doc, err := Doc(req.Content)
	h.reply(w, http.StatusOK, map[string]any{"doc": doc}, err)
}

func (h *Handler) generate(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	var req generateRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	if err := h.gen.Generate(w, r, id, req); err != nil {
		apperr.Write(w, h.logger, err)
	}
}

func (h *Handler) activeTemplates(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ActiveTemplates(r.Context())
	h.reply(w, http.StatusOK, map[string]any{"items": out}, err)
}

func (h *Handler) allTemplates(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.AllTemplates(r.Context(), authz.FromContext(r.Context()))
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

func (h *Handler) archiveTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	out, err := h.svc.ArchiveTemplate(r.Context(), authz.FromContext(r.Context()), id)
	h.reply(w, http.StatusOK, out, err)
}
