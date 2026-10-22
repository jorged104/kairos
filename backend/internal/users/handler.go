package users

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"openscum/internal/apperr"
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

// Routes require a session; each operation checks its own permission.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/users", h.List)
	r.Post("/users", h.Create)
	r.Get("/users/{id}", h.Get)
	r.Patch("/users/{id}", h.Update)
	r.Post("/users/{id}/deactivate", h.status(Deactivate))
	r.Post("/users/{id}/reactivate", h.status(Reactivate))
	r.Post("/users/{id}/offboard", h.status(Offboard))
	r.Post("/users/{id}/restore", h.status(Restore))
	r.Put("/users/{id}/password", h.SetPassword)
	r.Post("/users/{id}/roles", h.AddRole)
	r.Delete("/users/{id}/roles/{role}", h.RemoveRole)
	r.Post("/users/{id}/teams", h.AddTeam)
	r.Delete("/users/{id}/teams/{team_id}", h.RemoveTeam)
}

func (h *Handler) respond(w http.ResponseWriter, status int, u User, err error) {
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, status, u)
}

func (h *Handler) id(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, ok := httpx.PathUUID(r, name)
	if !ok {
		apperr.Write(w, h.logger, apperr.ErrNotFound)
	}
	return id, ok
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	u, err := h.svc.Create(r.Context(), authz.FromContext(r.Context()), req)
	h.respond(w, http.StatusCreated, u, err)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r, "id")
	if !ok {
		return
	}
	u, err := h.svc.Get(r.Context(), authz.FromContext(r.Context()), id)
	h.respond(w, http.StatusOK, u, err)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r, "id")
	if !ok {
		return
	}
	var req updateRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	u, err := h.svc.Update(r.Context(), authz.FromContext(r.Context()), id, req)
	h.respond(w, http.StatusOK, u, err)
}

func (h *Handler) status(tr Transition) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := h.id(w, r, "id")
		if !ok {
			return
		}
		u, err := h.svc.SetStatus(r.Context(), authz.FromContext(r.Context()), id, tr)
		h.respond(w, http.StatusOK, u, err)
	}
}

func (h *Handler) SetPassword(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r, "id")
	if !ok {
		return
	}
	var req passwordRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	if err := h.svc.SetPassword(r.Context(), authz.FromContext(r.Context()), id, req.Password); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) AddRole(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r, "id")
	if !ok {
		return
	}
	var req roleRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	u, err := h.svc.AddRole(r.Context(), authz.FromContext(r.Context()), id, req.Role)
	h.respond(w, http.StatusOK, u, err)
}

func (h *Handler) RemoveRole(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r, "id")
	if !ok {
		return
	}
	role, valid := authz.ParseRole(chi.URLParam(r, "role"))
	if !valid {
		apperr.Write(w, h.logger, apperr.ErrNotFound)
		return
	}
	u, err := h.svc.RemoveRole(r.Context(), authz.FromContext(r.Context()), id, string(role))
	h.respond(w, http.StatusOK, u, err)
}

func (h *Handler) AddTeam(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r, "id")
	if !ok {
		return
	}
	var req teamRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	u, err := h.svc.AddTeam(r.Context(), authz.FromContext(r.Context()), id, req.TeamID)
	h.respond(w, http.StatusOK, u, err)
}

func (h *Handler) RemoveTeam(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r, "id")
	if !ok {
		return
	}
	teamID, ok := h.id(w, r, "team_id")
	if !ok {
		return
	}
	u, err := h.svc.RemoveTeam(r.Context(), authz.FromContext(r.Context()), id, teamID)
	h.respond(w, http.StatusOK, u, err)
}

// List is the directory (RF-109 to RF-116).
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.Directory(r.Context(), authz.FromContext(r.Context()), r.URL.Query())
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}
