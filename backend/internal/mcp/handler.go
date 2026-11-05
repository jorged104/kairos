package mcp

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"openscum/internal/apperr"
	"openscum/internal/auth"
	"openscum/internal/authz"
	"openscum/pkg/httpx"
)

// Handler serves the connections of the account page and of the system
// administration (RF-8 to RF-25).
type Handler struct {
	conns  *Connections
	logger *slog.Logger
}

func NewHandler(conns *Connections, logger *slog.Logger) *Handler {
	return &Handler{conns: conns, logger: logger}
}

// Routes need a session; anybody manages their own connections, and
// system:manage everybody's.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/me/mcp-connections", h.mine)
	r.Post("/me/mcp-connections", h.create)
	r.Delete("/me/mcp-connections/{id}", h.revoke(false))
	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(authz.SystemManage, h.logger))
		r.Get("/system/mcp-connections", h.all)
		r.Delete("/system/mcp-connections/{id}", h.revoke(true))
	})
}

func (h *Handler) mine(w http.ResponseWriter, r *http.Request) {
	out, err := h.conns.Mine(r.Context(), authz.FromContext(r.Context()))
	h.reply(w, http.StatusOK, out, err)
}

func (h *Handler) all(w http.ResponseWriter, r *http.Request) {
	items, err := h.conns.All(r.Context(), authz.FromContext(r.Context()))
	h.reply(w, http.StatusOK, map[string]any{"items": items}, err)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	out, err := h.conns.Create(r.Context(), authz.FromContext(r.Context()), req)
	h.reply(w, http.StatusCreated, out, err)
}

func (h *Handler) revoke(others bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := httpx.PathUUID(r, "id")
		if !ok {
			apperr.Write(w, h.logger, apperr.ErrNotFound)
			return
		}
		if err := h.conns.Revoke(r.Context(), authz.FromContext(r.Context()), id, others); err != nil {
			apperr.Write(w, h.logger, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) reply(w http.ResponseWriter, status int, v any, err error) {
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, status, v)
}
