package teams

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

// Routes require a session; changes also require teams:manage.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/teams", h.List)
	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(authz.TeamsManage, h.logger))
		r.Post("/teams", h.Create)
		r.Patch("/teams/{id}", h.Update)
		r.Post("/teams/{id}/archive", h.archive(true))
		r.Post("/teams/{id}/unarchive", h.archive(false))
		r.Post("/teams/{id}/leaders", h.AddLeader)
		r.Delete("/teams/{id}/leaders/{user_id}", h.RemoveLeader)
	})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	teams, err := h.svc.List(r.Context())
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": teams})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req teamRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	t, err := h.svc.Create(r.Context(), authz.FromContext(r.Context()), req)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, t)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := h.pathID(w, r, "id")
	if !ok {
		return
	}
	var req teamRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	t, err := h.svc.Update(r.Context(), authz.FromContext(r.Context()), id, req)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, t)
}

func (h *Handler) archive(archived bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := h.pathID(w, r, "id")
		if !ok {
			return
		}
		t, err := h.svc.SetArchived(r.Context(), authz.FromContext(r.Context()), id, archived)
		if err != nil {
			apperr.Write(w, h.logger, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, t)
	}
}

func (h *Handler) AddLeader(w http.ResponseWriter, r *http.Request) {
	id, ok := h.pathID(w, r, "id")
	if !ok {
		return
	}
	var req leaderRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	if err := h.svc.AddLeader(r.Context(), authz.FromContext(r.Context()), id, req.UserID); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RemoveLeader(w http.ResponseWriter, r *http.Request) {
	id, ok := h.pathID(w, r, "id")
	if !ok {
		return
	}
	userID, ok := h.pathID(w, r, "user_id")
	if !ok {
		return
	}
	if err := h.svc.RemoveLeader(r.Context(), authz.FromContext(r.Context()), id, userID); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) pathID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		apperr.Write(w, h.logger, apperr.ErrNotFound)
		return id, false
	}
	return id, true
}
