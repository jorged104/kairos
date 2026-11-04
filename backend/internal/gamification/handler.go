package gamification

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

// Routes: everybody reads their own points, the tables and the rankings of
// their teams; the rest needs gamification:manage.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/me/points", h.summary(func(r *http.Request) (uuid.UUID, bool) { return authz.FromContext(r.Context()).UserID, true }))
	r.Get("/me/points/movements", h.history(func(r *http.Request) (uuid.UUID, bool) { return authz.FromContext(r.Context()).UserID, true }))
	r.Get("/gamification/values", h.values)
	r.Get("/gamification/levels", h.levels)
	r.Get("/gamification/teams/{id}/ranking", h.ranking)
	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(authz.GamificationManage, h.logger))
		r.Get("/gamification/users/{id}/points", h.summary(pathID))
		r.Get("/gamification/users/{id}/movements", h.history(pathID))
		r.Post("/gamification/users/{id}/adjustments", h.adjust)
		r.Put("/gamification/values", h.setValues)
		r.Post("/gamification/levels", h.createLevel)
		r.Patch("/gamification/levels/{id}", h.updateLevel)
		r.Delete("/gamification/levels/{id}", h.deleteLevel)
	})
}

func pathID(r *http.Request) (uuid.UUID, bool) { return httpx.PathUUID(r, "id") }

func (h *Handler) summary(user func(*http.Request) (uuid.UUID, bool)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := user(r)
		if !ok {
			apperr.Write(w, h.logger, apperr.ErrNotFound)
			return
		}
		out, err := h.svc.Summary(r.Context(), authz.FromContext(r.Context()), id)
		h.reply(w, out, err)
	}
}

// history reads ?page, or ?after_seq for the movements after a cursor.
func (h *Handler) history(user func(*http.Request) (uuid.UUID, bool)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := user(r)
		if !ok {
			apperr.Write(w, h.logger, apperr.ErrNotFound)
			return
		}
		q := r.URL.Query()
		page, ok := httpx.ParsePage(q, historyPageSize, historyPageSize)
		if !ok {
			apperr.Write(w, h.logger, apperr.ErrInvalidFilter.WithDetail("field", "page"))
			return
		}
		var after int64
		if v := q.Get("after_seq"); v != "" {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil || n < 0 {
				apperr.Write(w, h.logger, apperr.ErrInvalidFilter.WithDetail("field", "after_seq"))
				return
			}
			after = n
		}
		out, err := h.svc.History(r.Context(), authz.FromContext(r.Context()), id, page, after)
		h.reply(w, out, err)
	}
}

func (h *Handler) adjust(w http.ResponseWriter, r *http.Request) {
	id, _ := httpx.PathUUID(r, "id")
	var req AdjustRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	out, err := h.svc.Adjust(r.Context(), authz.FromContext(r.Context()), id, req)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, out)
}

func (h *Handler) values(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Values(r.Context())
	h.reply(w, out, err)
}

func (h *Handler) setValues(w http.ResponseWriter, r *http.Request) {
	var req SetValuesRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	out, err := h.svc.SetValues(r.Context(), authz.FromContext(r.Context()), req)
	h.reply(w, out, err)
}

func (h *Handler) levels(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Levels(r.Context())
	h.reply(w, map[string]any{"items": out}, err)
}

func (h *Handler) createLevel(w http.ResponseWriter, r *http.Request) {
	var req LevelRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	out, err := h.svc.CreateLevel(r.Context(), authz.FromContext(r.Context()), req)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, out)
}

func (h *Handler) updateLevel(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		apperr.Write(w, h.logger, apperr.ErrNotFound)
		return
	}
	var req LevelRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	out, err := h.svc.UpdateLevel(r.Context(), authz.FromContext(r.Context()), id, req)
	h.reply(w, out, err)
}

func (h *Handler) deleteLevel(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		apperr.Write(w, h.logger, apperr.ErrNotFound)
		return
	}
	if err := h.svc.DeleteLevel(r.Context(), authz.FromContext(r.Context()), id); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ranking reads ?period=week|month|all and ?today=YYYY-MM-DD, the caller's date.
func (h *Handler) ranking(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		apperr.Write(w, h.logger, apperr.ErrNotFound)
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
	out, err := h.svc.Ranking(r.Context(), authz.FromContext(r.Context()), id, r.URL.Query().Get("period"), today)
	h.reply(w, out, err)
}

func (h *Handler) reply(w http.ResponseWriter, out any, err error) {
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}
