package projectboard

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"openscum/internal/apperr"
	"openscum/internal/auth"
	"openscum/internal/authz"
	"openscum/pkg/httpx"
)

type Handler struct {
	board  *Service
	views  *Views
	logger *slog.Logger
}

func NewHandler(board *Service, views *Views, logger *slog.Logger) *Handler {
	return &Handler{board: board, views: views, logger: logger}
}

// Routes: whoever reads work sees every project and keeps personal views (RF-29, RF-70).
func (h *Handler) Routes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(authz.WorkRead, h.logger))
		r.Get("/project-board", h.get)
		r.Get("/project-views", h.list)
		r.Post("/project-views", h.create)
		r.Patch("/project-views/{id}", h.update)
		r.Delete("/project-views/{id}", h.delete)
	})
}

func (h *Handler) reply(w http.ResponseWriter, status int, v any, err error) {
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	if v == nil {
		w.WriteHeader(status)
		return
	}
	httpx.WriteJSON(w, status, v)
}

func (h *Handler) id(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		apperr.Write(w, h.logger, apperr.ErrNotFound)
	}
	return id, ok
}

// get reads ?today=YYYY-MM-DD, the caller's date, or uses the server's.
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	today := time.Now().UTC()
	if v := r.URL.Query().Get("today"); v != "" {
		t, err := time.Parse(time.DateOnly, v)
		if err != nil {
			apperr.Write(w, h.logger, apperr.ErrInvalidFilter.WithDetail("field", "today"))
			return
		}
		today = t
	}
	out, err := h.board.Get(r.Context(), authz.FromContext(r.Context()), today)
	h.reply(w, http.StatusOK, out, err)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	out, err := h.views.List(r.Context(), authz.FromContext(r.Context()))
	h.reply(w, http.StatusOK, map[string]any{"items": out}, err)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req request
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	out, err := h.views.Create(r.Context(), authz.FromContext(r.Context()), req)
	h.reply(w, http.StatusCreated, out, err)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	var req request
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	out, err := h.views.Update(r.Context(), authz.FromContext(r.Context()), id, req)
	h.reply(w, http.StatusOK, out, err)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	h.reply(w, http.StatusNoContent, nil, h.views.Delete(r.Context(), authz.FromContext(r.Context()), id))
}
