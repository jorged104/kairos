package audit

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"openscum/internal/apperr"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
	"openscum/pkg/httpx"
)

const (
	defaultPageSize = 50
	maxPageSize     = 200
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
	r.Get("/audit", h.List)
}

type entryResponse struct {
	ID         int64           `json:"id"`
	OccurredAt time.Time       `json:"occurred_at"`
	ActorID    *uuid.UUID      `json:"actor_id"`
	ActorKind  string          `json:"actor_kind"`
	ActorName  *string         `json:"actor_name"`
	Action     string          `json:"action"`
	TargetType string          `json:"target_type"`
	TargetID   string          `json:"target_id"`
	TargetName *string         `json:"target_name"`
	Before     json.RawMessage `json:"before"`
	After      json.RawMessage `json:"after"`
}

type listResponse struct {
	Items    []entryResponse `json:"items"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// List returns the audit log, newest first, filtered by affected user
// (?user_id), actor (?actor_id) and time range (?from, ?to in RFC 3339 or YYYY-MM-DD).
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	page, ok := httpx.ParsePage(r.URL.Query(), defaultPageSize, maxPageSize)
	if !ok {
		apperr.Write(w, h.logger, apperr.ErrInvalidFilter.WithDetail("field", "page"))
		return
	}
	f, err := parseFilters(r)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	rows, err := h.store.ListAudit(r.Context(), sqlc.ListAuditParams{
		TargetID: f.TargetID, ActorID: f.ActorID, FromTime: f.FromTime, ToTime: f.ToTime,
		RowLimit: int32(page.Size), RowOffset: int32(page.Offset()),
	})
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	total, err := h.store.CountAudit(r.Context(), f)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	res := listResponse{Items: make([]entryResponse, 0, len(rows)), Total: total, Page: page.Number, PageSize: page.Size}
	for _, e := range rows {
		res.Items = append(res.Items, entryResponse{
			ID: e.ID, OccurredAt: e.OccurredAt, ActorID: e.ActorID, ActorKind: e.ActorKind, ActorName: e.ActorName,
			Action: e.Action, TargetType: e.TargetType, TargetID: e.TargetID, TargetName: e.TargetName,
			Before: e.Before, After: e.After,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func parseFilters(r *http.Request) (sqlc.CountAuditParams, error) {
	var f sqlc.CountAuditParams
	q := r.URL.Query()
	if v := q.Get("user_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return f, apperr.ErrInvalidFilter.WithDetail("field", "user_id")
		}
		s := id.String()
		f.TargetID = &s
	}
	if v := q.Get("actor_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return f, apperr.ErrInvalidFilter.WithDetail("field", "actor_id")
		}
		f.ActorID = &id
	}
	for name, dst := range map[string]**time.Time{"from": &f.FromTime, "to": &f.ToTime} {
		v := q.Get(name)
		if v == "" {
			continue
		}
		t, err := parseTime(v, name == "to")
		if err != nil {
			return f, apperr.ErrInvalidFilter.WithDetail("field", name)
		}
		*dst = &t
	}
	return f, nil
}

// parseTime accepts RFC 3339 or a date; a date used as upper bound includes that whole day.
func parseTime(v string, upper bool) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, nil
	}
	t, err := time.Parse(time.DateOnly, v)
	if err != nil {
		return t, err
	}
	if upper {
		t = t.AddDate(0, 0, 1)
	}
	return t, nil
}
