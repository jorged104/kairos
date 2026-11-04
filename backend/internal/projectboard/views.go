package projectboard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/auth"
	"openscum/internal/authz"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
)

const (
	MaxPersonal = 50 // per person (RF-77)
	MaxShared   = 30 // for the whole organization
	maxValues   = 50
)

// Config is what a view keeps: the mode, the filters, the order, the columns
// of the Kanban and the grouping of the Timeline (RF-70).
type Config struct {
	Mode    string              `json:"mode"`
	Filters map[string][]string `json:"filters"`
	Sort    string              `json:"sort"`
	Columns string              `json:"columns"`
	Group   string              `json:"group"`
}

var (
	modes   = []string{"", "kanban", "timeline", "table"}
	sorts   = withDesc("gap", "end", "name", "real", "identifier", "status", "program", "owner", "health", "plan", "start")
	columns = []string{"", "status", "level", "health", "program"}
	groups  = []string{"", "program", "status", "owner", "none"}
	// Filter fields and how each value is checked (RF-62 to RF-64).
	fields = map[string]func(string) bool{
		"status_id":  isID,
		"level":      oneOf("ahead", "on_track", "at_risk", "late", "none"),
		"health":     oneOf("en_curso", "en_riesgo", "fuera_de_curso"),
		"program_id": func(v string) bool { return v == "none" || isID(v) },
		"team_id":    isID,
		"owner_id":   func(v string) bool { return v == "me" || isID(v) },
		"end":        oneOf("overdue", "month", "quarter", "none"),
		"end_from":   isDate,
		"end_to":     isDate,
		"q":          func(v string) bool { return len(v) <= 200 },
	}
)

// withDesc lists the orders, each ascending and with «-» descending (RF-61, RF-68).
func withDesc(keys ...string) []string {
	out := []string{""}
	for _, k := range keys {
		out = append(out, k, "-"+k)
	}
	return out
}

func isID(v string) bool { _, err := uuid.Parse(v); return err == nil }

func isDate(v string) bool { _, err := time.Parse(time.DateOnly, v); return err == nil }

func oneOf(vs ...string) func(string) bool {
	return func(v string) bool { return slices.Contains(vs, v) }
}

// check validates a config: known mode, filters, order, columns and grouping.
func check(cfg *Config) error {
	bad := func(field string) error { return apperr.ErrValidation.WithDetail("field", field) }
	if cfg.Filters == nil {
		cfg.Filters = map[string][]string{}
	}
	for field, values := range cfg.Filters {
		ok := fields[field]
		if ok == nil || len(values) == 0 || len(values) > maxValues {
			return bad("filters." + field)
		}
		for _, v := range values {
			if !ok(v) {
				return bad("filters." + field)
			}
		}
	}
	switch {
	case !slices.Contains(modes, cfg.Mode):
		return bad("mode")
	case !slices.Contains(sorts, cfg.Sort):
		return bad("sort")
	case !slices.Contains(columns, cfg.Columns):
		return bad("columns")
	case !slices.Contains(groups, cfg.Group):
		return bad("group")
	}
	return nil
}

// View as each person sees it; the browser counts its projects (RF-72).
type View struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	Shared  bool      `json:"shared"`
	Mine    bool      `json:"mine"`
	CanEdit bool      `json:"can_edit"`
	Config  Config    `json:"config"`
}

type request struct {
	Name   string `json:"name" validate:"required,max=100"`
	Shared bool   `json:"shared"`
	Config Config `json:"config"`
}

func (r *request) Normalize() { r.Name = strings.TrimSpace(r.Name) }

var (
	errNameConflict = apperr.New(http.StatusConflict, "view_name_conflict", "Ya hay una vista con ese nombre")
	errTooMany      = apperr.New(http.StatusConflict, "too_many_views", "Se ha llegado al máximo de vistas")
)

type Views struct{ store *db.Store }

func NewViews(store *db.Store) *Views { return &Views{store: store} }

func toView(p *authz.Principal, r sqlc.ProjectListView) (View, error) {
	var cfg Config
	if err := json.Unmarshal(r.Config, &cfg); err != nil {
		return View{}, fmt.Errorf("leer vista: %w", err)
	}
	if cfg.Filters == nil {
		cfg.Filters = map[string][]string{}
	}
	return View{ID: r.ID, Name: r.Name, Shared: r.OwnerID == nil, Mine: r.OwnerID != nil && *r.OwnerID == p.UserID,
		CanEdit: canEdit(p, r), Config: cfg}, nil
}

// canEdit: a personal view, its author; a shared one, who shares views (RF-75).
func canEdit(p *authz.Principal, r sqlc.ProjectListView) bool {
	if r.OwnerID == nil {
		return p.Has(authz.ProjectViewsShare)
	}
	return *r.OwnerID == p.UserID
}

// List returns the shared views and the caller's (RF-72).
func (s *Views) List(ctx context.Context, p *authz.Principal) ([]View, error) {
	rows, err := s.store.ListVisibleListViews(ctx, p.UserID)
	if err != nil {
		return nil, fmt.Errorf("leer vistas: %w", err)
	}
	out := []View{}
	for _, r := range rows {
		v, err := toView(p, r)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func ownerFor(p *authz.Principal, shared bool) (*uuid.UUID, error) {
	if !shared {
		id := p.UserID
		return &id, nil
	}
	if !p.Has(authz.ProjectViewsShare) { // RF-71
		return nil, apperr.ErrForbidden
	}
	return nil, nil
}

func (s *Views) Create(ctx context.Context, p *authz.Principal, req request) (View, error) {
	var out View
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		owner, err := ownerFor(p, req.Shared)
		if err != nil {
			return err
		}
		if err := check(&req.Config); err != nil {
			return err
		}
		if err := room(ctx, q, owner); err != nil {
			return err
		}
		if err := nameFree(ctx, q, req.Name, owner, uuid.Nil); err != nil {
			return err
		}
		cfg, _ := json.Marshal(req.Config)
		row, err := q.CreateListView(ctx, sqlc.CreateListViewParams{OwnerID: owner, Name: req.Name,
			NameNormalized: normalize(req.Name), Config: cfg, CreatedBy: p.UserID})
		if auth.IsUniqueViolation(err) {
			return errNameConflict
		}
		if err != nil {
			return fmt.Errorf("guardar vista: %w", err)
		}
		if err := record(ctx, q, p, row, false, "project_list_view.created"); err != nil {
			return err
		}
		out, err = toView(p, row)
		return err
	})
	return out, err
}

func (s *Views) Update(ctx context.Context, p *authz.Principal, id uuid.UUID, req request) (View, error) {
	var out View
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		row, err := load(ctx, q, p, id)
		if err != nil {
			return err
		}
		owner, err := ownerFor(p, req.Shared)
		if err != nil {
			return err
		}
		if err := check(&req.Config); err != nil {
			return err
		}
		if (owner == nil) != (row.OwnerID == nil) {
			if err := room(ctx, q, owner); err != nil {
				return err
			}
		}
		if err := nameFree(ctx, q, req.Name, owner, id); err != nil {
			return err
		}
		cfg, _ := json.Marshal(req.Config)
		next, err := q.UpdateListView(ctx, sqlc.UpdateListViewParams{ID: id, Name: req.Name, NameNormalized: normalize(req.Name), Config: cfg, OwnerID: owner})
		if auth.IsUniqueViolation(err) {
			return errNameConflict
		}
		if err != nil {
			return fmt.Errorf("guardar vista: %w", err)
		}
		if err := record(ctx, q, p, next, row.OwnerID == nil, "project_list_view.updated"); err != nil {
			return err
		}
		out, err = toView(p, next)
		return err
	})
	return out, err
}

func (s *Views) Delete(ctx context.Context, p *authz.Principal, id uuid.UUID) error {
	return s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		row, err := load(ctx, q, p, id)
		if err != nil {
			return err
		}
		if err := q.DeleteListView(ctx, id); err != nil {
			return fmt.Errorf("borrar vista: %w", err)
		}
		return record(ctx, q, p, row, false, "project_list_view.deleted")
	})
}

// load reads a view the caller can change; someone else's personal view is not found.
func load(ctx context.Context, q *sqlc.Queries, p *authz.Principal, id uuid.UUID) (sqlc.ProjectListView, error) {
	row, err := q.GetListView(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.OwnerID != nil && *row.OwnerID != p.UserID) {
		return row, apperr.ErrNotFound
	}
	if err != nil {
		return row, fmt.Errorf("leer vista: %w", err)
	}
	if !canEdit(p, row) {
		return row, apperr.ErrForbidden
	}
	return row, nil
}

func room(ctx context.Context, q *sqlc.Queries, owner *uuid.UUID) error {
	n, err := q.CountListViews(ctx, owner)
	if err != nil {
		return fmt.Errorf("contar vistas: %w", err)
	}
	if (owner == nil && n >= MaxShared) || (owner != nil && n >= MaxPersonal) {
		return errTooMany
	}
	return nil
}

// nameFree: unique among the views someone sees, «Todos» included (RF-76).
func nameFree(ctx context.Context, q *sqlc.Queries, name string, owner *uuid.UUID, except uuid.UUID) error {
	if normalize(name) == "todos" {
		return errNameConflict
	}
	taken, err := q.ListViewNameTaken(ctx, sqlc.ListViewNameTakenParams{NameNormalized: normalize(name), ExceptID: except, OwnerID: owner})
	if err != nil {
		return fmt.Errorf("comprobar nombre: %w", err)
	}
	if taken {
		return errNameConflict
	}
	return nil
}

func normalize(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

// record audits the changes of shared views, and a view that stops being
// shared (RF-80); personal views are not audited.
func record(ctx context.Context, q *sqlc.Queries, p *authz.Principal, row sqlc.ProjectListView, wasShared bool, action string) error {
	if row.OwnerID != nil && !wasShared {
		return nil
	}
	return audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: action, TargetType: "project_list_view", TargetID: row.ID.String(),
		After: map[string]any{"name": row.Name, "shared": row.OwnerID == nil}})
}
