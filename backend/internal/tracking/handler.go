package tracking

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
	svc    *Service
	logger *slog.Logger
}

func NewHandler(svc *Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

// Routes: administrators, Dirección and leads follow the projects they see
// (spec 023, RF-17a).
func (h *Handler) Routes(r chi.Router) {
	r.With(auth.RequirePermission(authz.OversightView, h.logger)).Get("/tracking", h.get)
}

// get reads ?program_id=&project_id=…&owner_id=…&today=YYYY-MM-DD; ids that
// are not valid are rejected, ids that do not exist are ignored.
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var f Filters
	ids := func(field string) ([]uuid.UUID, bool) {
		var out []uuid.UUID
		for _, v := range q[field] {
			id, err := uuid.Parse(v)
			if err != nil {
				apperr.Write(w, h.logger, apperr.ErrInvalidFilter.WithDetail("field", field))
				return nil, false
			}
			out = append(out, id)
		}
		return out, true
	}
	program, ok := ids("program_id")
	if !ok {
		return
	}
	if len(program) > 0 {
		f.ProgramID = &program[0]
	}
	if f.ProjectIDs, ok = ids("project_id"); !ok {
		return
	}
	if f.OwnerIDs, ok = ids("owner_id"); !ok {
		return
	}
	today := time.Now().UTC()
	if v := q.Get("today"); v != "" {
		t, err := time.Parse(time.DateOnly, v)
		if err != nil {
			apperr.Write(w, h.logger, apperr.ErrInvalidFilter.WithDetail("field", "today"))
			return
		}
		today = t
	}
	out, err := h.svc.Get(r.Context(), authz.FromContext(r.Context()), f, today)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}
