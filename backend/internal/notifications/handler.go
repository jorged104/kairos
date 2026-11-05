package notifications

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

// Routes need work:read: every notification is about the work.
func (h *Handler) Routes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(authz.WorkRead, h.logger))
		r.Get("/notifications", h.list)
		r.Get("/notifications/unread", h.unread)
		r.Put("/notifications/{id}/read", h.read)
		r.Post("/notifications/read-all", h.readAll)
		r.Get("/work-items/{id}/follow", h.following)
		r.Put("/work-items/{id}/follow", h.setFollowing)
	})
}

type readRequest struct {
	Read bool `json:"read"`
}

type followRequest struct {
	Following bool `json:"following"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, ok := httpx.ParsePage(q, PageSize, PageSize)
	if !ok {
		apperr.Write(w, h.logger, apperr.ErrInvalidFilter.WithDetail("field", "page"))
		return
	}
	out, err := h.svc.List(r.Context(), authz.FromContext(r.Context()), q.Get("unread") == "true", page.Number, page.Size)
	h.reply(w, out, err)
}

func (h *Handler) unread(w http.ResponseWriter, r *http.Request) {
	n, err := h.svc.Unread(r.Context(), authz.FromContext(r.Context()))
	h.reply(w, map[string]int64{"unread": n}, err)
}

func (h *Handler) read(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		apperr.Write(w, h.logger, apperr.ErrNotFound)
		return
	}
	var req readRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	h.done(w, h.svc.MarkRead(r.Context(), authz.FromContext(r.Context()), id, req.Read))
}

func (h *Handler) readAll(w http.ResponseWriter, r *http.Request) {
	h.done(w, h.svc.ReadAll(r.Context(), authz.FromContext(r.Context())))
}

func (h *Handler) following(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		apperr.Write(w, h.logger, apperr.ErrNotFound)
		return
	}
	following, err := h.svc.Following(r.Context(), authz.FromContext(r.Context()), id)
	h.reply(w, map[string]bool{"following": following}, err)
}

func (h *Handler) setFollowing(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		apperr.Write(w, h.logger, apperr.ErrNotFound)
		return
	}
	var req followRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	if err := h.svc.SetFollowing(r.Context(), authz.FromContext(r.Context()), id, req.Following); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"following": req.Following})
}

func (h *Handler) reply(w http.ResponseWriter, v any, err error) {
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, v)
}

func (h *Handler) done(w http.ResponseWriter, err error) {
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
