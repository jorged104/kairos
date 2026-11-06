package settings

import (
	"context"
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

// Routes need system:manage (RF-1, RF-2).
func (h *Handler) Routes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(authz.SystemManage, h.logger))
		r.Get("/system/settings", h.view)
		r.Put("/system/settings/access", put(h, h.svc.UpdateAccess))
		r.Put("/system/settings/sso", put(h, h.svc.UpdateSSO))
		r.Put("/system/settings/storage", put(h, h.svc.UpdateStorage))
		r.Put("/system/settings/appearance", put(h, h.svc.UpdateAppearance))
		r.Put("/system/settings/mcp", put(h, h.svc.UpdateMCP))
		r.Put("/system/settings/mail", put(h, h.svc.UpdateMail))
		r.Post("/system/settings/mail/test", h.testMail)
		r.Put("/system/settings/ai", put(h, h.svc.UpdateAI))
		r.Post("/system/settings/ai/test", h.testAI)
	})
}

func (h *Handler) view(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.View(r.Context())
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func put[Req any](h *Handler, fn func(context.Context, *authz.Principal, Req) (View, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req Req
		if err := apperr.Decode(r, &req); err != nil {
			apperr.Write(w, h.logger, err)
			return
		}
		out, err := fn(r.Context(), authz.FromContext(r.Context()), req)
		if err != nil {
			apperr.Write(w, h.logger, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, out)
	}
}

// testMail sends the test mail (spec 019, RF-6).
func (h *Handler) testMail(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.TestMail(r.Context(), authz.FromContext(r.Context())); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// testAI checks the AI service of the reports (spec 020, RF-11).
func (h *Handler) testAI(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.TestAI(r.Context(), authz.FromContext(r.Context())); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
