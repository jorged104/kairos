package mentions

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"openscum/internal/apperr"
	"openscum/internal/auth"
	"openscum/internal/authz"
	"openscum/internal/db"
	"openscum/pkg/httpx"
)

type Handler struct {
	store  *db.Store
	logger *slog.Logger
}

func NewHandler(store *db.Store, logger *slog.Logger) *Handler {
	return &Handler{store: store, logger: logger}
}

func (h *Handler) Routes(r chi.Router) {
	r.With(auth.RequirePermission(authz.WorkRead, h.logger)).Get("/mentions", h.search)
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	out, err := Search(r.Context(), h.store.Queries, authz.FromContext(r.Context()), r.URL.Query().Get("q"))
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}
