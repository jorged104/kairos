package accounts

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"openscum/internal/apperr"
	"openscum/pkg/httpx"
)

// Handler serves the public side of the links: asking for one from the login
// and using it. The secret always travels in the body, never in the URL (RF-39).
type Handler struct {
	svc    *Service
	logger *slog.Logger
}

func NewHandler(svc *Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

func (h *Handler) PublicRoutes(r chi.Router) {
	r.Post("/auth/password/forgot", h.forgot)
	r.Post("/auth/password/check", h.check)
	r.Post("/auth/password/reset", h.reset)
}

type forgotRequest struct {
	Email string `json:"email" validate:"max=320"`
}

type tokenRequest struct {
	Token string `json:"token" validate:"required,max=100"`
}

type resetRequest struct {
	Token    string `json:"token" validate:"required,max=100"`
	Password string `json:"password"`
}

// forgot answers the same whatever the address (RF-25).
func (h *Handler) forgot(w http.ResponseWriter, r *http.Request) {
	var req forgotRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	if err := h.svc.Forgot(r.Context(), req.Email); err != nil {
		h.logger.Error("recuperar contraseña", slog.Any("error", err))
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) check(w http.ResponseWriter, r *http.Request) {
	var req tokenRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	out, err := h.svc.Check(r.Context(), req.Token)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) reset(w http.ResponseWriter, r *http.Request) {
	var req resetRequest
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	if err := h.svc.Reset(r.Context(), req.Token, req.Password); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
