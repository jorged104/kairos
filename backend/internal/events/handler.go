package events

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"openscum/internal/apperr"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
	"openscum/pkg/httpx"
)

type Handler struct {
	store  *db.Store
	logger *slog.Logger
}

func NewHandler(store *db.Store, logger *slog.Logger) *Handler {
	return &Handler{store: store, logger: logger}
}

// Routes must be mounted behind the audit:read permission.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/events", h.List)
}

type listResponse struct {
	Items    []Event `json:"items"`
	Total    int64   `json:"total"`
	Page     int     `json:"page"`
	PageSize int     `json:"page_size"`
}

// List returns the recorded events, newest first, filtered by ?type and ?subject_id.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, ok := httpx.ParsePage(q, 50, 200)
	if !ok {
		apperr.Write(w, h.logger, apperr.ErrInvalidFilter.WithDetail("field", "page"))
		return
	}
	var typ *string
	if v := q.Get("type"); v != "" {
		typ = &v
	}
	var subject *uuid.UUID
	if v := q.Get("subject_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			apperr.Write(w, h.logger, apperr.ErrInvalidFilter.WithDetail("field", "subject_id"))
			return
		}
		subject = &id
	}
	rows, err := h.store.ListDomainEvents(r.Context(), sqlc.ListDomainEventsParams{
		Limit: int32(page.Size), Offset: int32(page.Offset()), Type: typ, SubjectID: subject,
	})
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	total, err := h.store.CountDomainEvents(r.Context(), sqlc.CountDomainEventsParams{Type: typ, SubjectID: subject})
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	res := listResponse{Items: make([]Event, 0, len(rows)), Total: total, Page: page.Number, PageSize: page.Size}
	for _, e := range rows {
		res.Items = append(res.Items, fromRow(e))
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}
