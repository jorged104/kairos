// Package views keeps the saved views of a project: filters, order and
// grouping for the table, the kanban and the timeline (spec 008, RF-23 to RF-31).
package views

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/auth"
	"openscum/internal/authz"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
	"openscum/internal/projects"
	"openscum/internal/workitems"
	"openscum/pkg/httpx"
)

const (
	MaxPersonal = 50 // per person and project (RF-30)
	MaxShared   = 30 // per project
	maxFilters  = 30
	maxValues   = 50
)

// Config is what a view keeps: the filters as the list's query parameters
// (workitems.FilterParams), the order and the grouping.
type Config struct {
	Filters map[string][]string `json:"filters"`
	Sort    string              `json:"sort"`
	Group   string              `json:"group"`
}

var groups = []string{"", "milestone", "status", "assignee", "type"}

// View as each person sees it.
type View struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	Shared  bool      `json:"shared"`
	Mine    bool      `json:"mine"`
	CanEdit bool      `json:"can_edit"`
	Config  Config    `json:"config"`
	Count   int64     `json:"count"`
	// Filter values that no longer exist or are retired (RF-31).
	Unavailable []Value `json:"unavailable"`
}

type Value struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

type List struct {
	Items []View `json:"items"`
	// Every work item of the project: the view «Todas» (RF-23).
	AllCount int64 `json:"all_count"`
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

type Service struct {
	store *db.Store
	items *workitems.Service
}

func NewService(store *db.Store, items *workitems.Service) *Service {
	return &Service{store: store, items: items}
}

// List returns «Todas», the shared views and the caller's, with their counts (RF-26).
func (s *Service) List(ctx context.Context, p *authz.Principal, projectID uuid.UUID) (List, error) {
	q := s.store.Queries
	project, err := projects.Get(ctx, q, p, projectID, false)
	if err != nil {
		return List{}, err
	}
	rows, err := q.ListVisibleViews(ctx, sqlc.ListVisibleViewsParams{ProjectID: projectID, UserID: p.UserID})
	if err != nil {
		return List{}, fmt.Errorf("leer vistas: %w", err)
	}
	ok, err := availability(ctx, q, projectID)
	if err != nil {
		return List{}, err
	}
	out := List{Items: []View{}}
	if out.AllCount, err = s.items.Count(ctx, p, project, url.Values{}); err != nil {
		return out, err
	}
	for _, r := range rows {
		v, err := s.view(ctx, p, project, r, ok)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, v)
	}
	return out, nil
}

func (s *Service) view(ctx context.Context, p *authz.Principal, project sqlc.Project, r sqlc.ProjectView, ok func(string, string) bool) (View, error) {
	var cfg Config
	if err := json.Unmarshal(r.Config, &cfg); err != nil {
		return View{}, fmt.Errorf("leer vista: %w", err)
	}
	if cfg.Filters == nil {
		cfg.Filters = map[string][]string{}
	}
	v := View{ID: r.ID, Name: r.Name, Shared: r.OwnerID == nil, Mine: r.OwnerID != nil && *r.OwnerID == p.UserID, Config: cfg,
		Unavailable: []Value{}}
	v.CanEdit = canEdit(p, project, r)
	query := url.Values{}
	for field, values := range cfg.Filters {
		for _, value := range values {
			if ok(field, value) {
				query.Add(field, value)
			} else {
				v.Unavailable = append(v.Unavailable, Value{Field: field, Value: value})
			}
		}
	}
	n, err := s.items.Count(ctx, p, project, query)
	v.Count = n
	return v, err
}

// canEdit: a personal view, its author; a shared one, whoever manages the project's team (RF-28).
func canEdit(p *authz.Principal, project sqlc.Project, r sqlc.ProjectView) bool {
	if r.OwnerID == nil {
		return p.CanManageProject(projects.Ref(project))
	}
	return *r.OwnerID == p.UserID
}

// availability says whether a filter value still exists and isn't retired (RF-31).
func availability(ctx context.Context, q *sqlc.Queries, projectID uuid.UUID) (func(field, value string) bool, error) {
	known := map[string]map[string]bool{}
	add := func(field string, id uuid.UUID, retired bool) {
		if known[field] == nil {
			known[field] = map[string]bool{}
		}
		if !retired {
			known[field][id.String()] = true
		}
	}
	statuses, err := q.ListStatuses(ctx)
	if err != nil {
		return nil, fmt.Errorf("leer estados: %w", err)
	}
	for _, r := range statuses {
		add("status_id", r.ID, r.RetiredAt != nil)
	}
	types, err := q.ListWorkItemTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("leer tipos: %w", err)
	}
	for _, r := range types {
		add("type_id", r.ID, r.RetiredAt != nil)
	}
	priorities, err := q.ListPriorities(ctx)
	if err != nil {
		return nil, fmt.Errorf("leer prioridades: %w", err)
	}
	for _, r := range priorities {
		add("priority_id", r.ID, r.RetiredAt != nil)
	}
	sizes, err := q.ListWorkItemSizes(ctx)
	if err != nil {
		return nil, fmt.Errorf("leer tallas: %w", err)
	}
	for _, r := range sizes {
		add("size_id", r.ID, r.RetiredAt != nil)
	}
	labels, err := q.ListLabels(ctx)
	if err != nil {
		return nil, fmt.Errorf("leer etiquetas: %w", err)
	}
	for _, r := range labels {
		add("label_id", r.ID, r.RetiredAt != nil)
	}
	milestones, err := q.ListProjectMilestones(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("leer hitos: %w", err)
	}
	for _, r := range milestones {
		add("milestone_id", r.ID, r.Status == "cancelado")
	}
	return func(field, value string) bool {
		switch field {
		case "status_id", "type_id", "priority_id", "label_id":
			return known[field][value]
		case "size_id", "milestone_id":
			return value == "none" || known[field][value]
		case "assignee_id":
			if value == "none" || value == "me" {
				return true
			}
			id, err := uuid.Parse(value)
			if err != nil {
				return false
			}
			u, err := q.GetUser(ctx, id)
			return err == nil && u.Status != "offboarded"
		default:
			return true
		}
	}, nil
}

// check validates a config: known filters, at most 30 of them with 50 values
// each, values that parse, a known order and grouping.
func check(project sqlc.Project, me uuid.UUID, cfg *Config) error {
	if cfg.Filters == nil {
		cfg.Filters = map[string][]string{}
	}
	bad := func(field string) error { return apperr.ErrValidation.WithDetail("field", field) }
	if len(cfg.Filters) > maxFilters {
		return bad("filters")
	}
	query := url.Values{}
	for field, values := range cfg.Filters {
		if !slices.Contains(workitems.FilterParams, field) || len(values) == 0 || len(values) > maxValues {
			return bad("filters." + field)
		}
		for _, v := range values {
			if len(v) > 200 {
				return bad("filters." + field)
			}
			query.Add(field, v)
		}
	}
	if _, err := workitems.Where(project.ID, project.Identifier, me, query); err != nil {
		return bad("filters")
	}
	if cfg.Sort != "" {
		query.Set("sort", cfg.Sort)
		if _, err := workitems.OrderBy(query, ""); err != nil {
			return bad("sort")
		}
	}
	if !slices.Contains(groups, cfg.Group) {
		return bad("group")
	}
	return nil
}

// Create saves a personal view, or a shared one for who manages the project (RF-24, RF-25).
func (s *Service) Create(ctx context.Context, p *authz.Principal, projectID uuid.UUID, req request) (View, error) {
	var out View
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		project, err := projects.Get(ctx, q, p, projectID, true)
		if err != nil {
			return err
		}
		owner, err := ownerFor(p, project, req.Shared)
		if err != nil {
			return err
		}
		if err := check(project, p.UserID, &req.Config); err != nil {
			return err
		}
		n, err := q.CountViews(ctx, sqlc.CountViewsParams{ProjectID: projectID, OwnerID: owner})
		if err != nil {
			return fmt.Errorf("contar vistas: %w", err)
		}
		if (owner == nil && n >= MaxShared) || (owner != nil && n >= MaxPersonal) {
			return errTooMany
		}
		if err := nameFree(ctx, q, projectID, req.Name, owner, uuid.Nil); err != nil {
			return err
		}
		cfg, _ := json.Marshal(req.Config)
		row, err := q.CreateView(ctx, sqlc.CreateViewParams{ProjectID: projectID, OwnerID: owner, Name: req.Name,
			NameNormalized: normalize(req.Name), Config: cfg, CreatedBy: p.UserID})
		if auth.IsUniqueViolation(err) {
			return errNameConflict
		}
		if err != nil {
			return fmt.Errorf("guardar vista: %w", err)
		}
		if err := record(ctx, q, p, row, "project.view_created"); err != nil {
			return err
		}
		ok, err := availability(ctx, q, projectID)
		if err != nil {
			return err
		}
		out, err = s.view(ctx, p, project, row, ok)
		return err
	})
	return out, err
}

// Update renames a view, changes its config or whether it is shared (RF-27, RF-28).
func (s *Service) Update(ctx context.Context, p *authz.Principal, id uuid.UUID, req request) (View, error) {
	var out View
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		row, project, err := s.load(ctx, q, p, id)
		if err != nil {
			return err
		}
		owner, err := ownerFor(p, project, req.Shared)
		if err != nil {
			return err
		}
		if err := check(project, p.UserID, &req.Config); err != nil {
			return err
		}
		if (owner == nil) != (row.OwnerID == nil) {
			n, err := q.CountViews(ctx, sqlc.CountViewsParams{ProjectID: project.ID, OwnerID: owner})
			if err != nil {
				return fmt.Errorf("contar vistas: %w", err)
			}
			if (owner == nil && n >= MaxShared) || (owner != nil && n >= MaxPersonal) {
				return errTooMany
			}
		}
		if err := nameFree(ctx, q, project.ID, req.Name, owner, id); err != nil {
			return err
		}
		cfg, _ := json.Marshal(req.Config)
		row, err = q.UpdateView(ctx, sqlc.UpdateViewParams{ID: id, Name: req.Name, NameNormalized: normalize(req.Name), Config: cfg, OwnerID: owner})
		if auth.IsUniqueViolation(err) {
			return errNameConflict
		}
		if err != nil {
			return fmt.Errorf("guardar vista: %w", err)
		}
		if err := record(ctx, q, p, row, "project.view_updated"); err != nil {
			return err
		}
		ok, err := availability(ctx, q, project.ID)
		if err != nil {
			return err
		}
		out, err = s.view(ctx, p, project, row, ok)
		return err
	})
	return out, err
}

func (s *Service) Delete(ctx context.Context, p *authz.Principal, id uuid.UUID) error {
	return s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		row, _, err := s.load(ctx, q, p, id)
		if err != nil {
			return err
		}
		if err := q.DeleteView(ctx, id); err != nil {
			return fmt.Errorf("borrar vista: %w", err)
		}
		return record(ctx, q, p, row, "project.view_deleted")
	})
}

// load reads a view the caller can change; someone else's personal view is not found.
func (s *Service) load(ctx context.Context, q *sqlc.Queries, p *authz.Principal, id uuid.UUID) (sqlc.ProjectView, sqlc.Project, error) {
	row, err := q.GetView(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.OwnerID != nil && *row.OwnerID != p.UserID) {
		return row, sqlc.Project{}, apperr.ErrNotFound
	}
	if err != nil {
		return row, sqlc.Project{}, fmt.Errorf("leer vista: %w", err)
	}
	project, err := projects.Get(ctx, q, p, row.ProjectID, true)
	if err != nil {
		return row, project, err
	}
	if !canEdit(p, project, row) {
		return row, project, apperr.ErrForbidden
	}
	return row, project, nil
}

func ownerFor(p *authz.Principal, project sqlc.Project, shared bool) (*uuid.UUID, error) {
	if !shared {
		id := p.UserID
		return &id, nil
	}
	if !p.CanManageProject(projects.Ref(project)) {
		return nil, apperr.ErrForbidden
	}
	return nil, nil
}

// nameFree: unique among the views someone sees, «Todas» included (RF-29).
func nameFree(ctx context.Context, q *sqlc.Queries, projectID uuid.UUID, name string, owner *uuid.UUID, except uuid.UUID) error {
	if normalize(name) == "todas" {
		return errNameConflict
	}
	taken, err := q.ViewNameTaken(ctx, sqlc.ViewNameTakenParams{ProjectID: projectID, NameNormalized: normalize(name), ExceptID: except, OwnerID: owner})
	if err != nil {
		return fmt.Errorf("comprobar nombre: %w", err)
	}
	if taken {
		return errNameConflict
	}
	return nil
}

func normalize(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

func record(ctx context.Context, q *sqlc.Queries, p *authz.Principal, row sqlc.ProjectView, action string) error {
	return audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: action, TargetType: "project", TargetID: row.ProjectID.String(),
		After: map[string]any{"view_id": row.ID, "name": row.Name, "shared": row.OwnerID == nil}})
}

// Handler ---------------------------------------------------------------------------

type Handler struct {
	svc    *Service
	logger *slog.Logger
}

func NewHandler(svc *Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

func (h *Handler) Routes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(authz.WorkRead, h.logger))
		r.Get("/projects/{id}/views", h.list)
		r.Post("/projects/{id}/views", h.create)
		r.Patch("/views/{id}", h.update)
		r.Delete("/views/{id}", h.delete)
	})
}

func (h *Handler) reply(w http.ResponseWriter, status int, v any, err error) {
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	if v == nil {
		w.WriteHeader(status)
		return
	}
	httpx.WriteJSON(w, status, v)
}

func (h *Handler) id(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := httpx.PathUUID(r, "id")
	if !ok {
		apperr.Write(w, h.logger, apperr.ErrNotFound)
	}
	return id, ok
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	out, err := h.svc.List(r.Context(), authz.FromContext(r.Context()), id)
	h.reply(w, http.StatusOK, out, err)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	var req request
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	out, err := h.svc.Create(r.Context(), authz.FromContext(r.Context()), id, req)
	h.reply(w, http.StatusCreated, out, err)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	var req request
	if err := apperr.Decode(r, &req); err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	out, err := h.svc.Update(r.Context(), authz.FromContext(r.Context()), id, req)
	h.reply(w, http.StatusOK, out, err)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := h.id(w, r)
	if !ok {
		return
	}
	h.reply(w, http.StatusNoContent, nil, h.svc.Delete(r.Context(), authz.FromContext(r.Context()), id))
}
