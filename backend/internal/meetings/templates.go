package meetings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/auth"
	"openscum/internal/authz"
	"openscum/internal/db/sqlc"
	"openscum/internal/files"
	"openscum/internal/projects"
)

var errTemplateName = apperr.New(http.StatusConflict, "meeting_template_name_taken", "Ya hay una plantilla con ese nombre")

// Template is a model for new meetings: global, or of one project (RF-29 to RF-32).
type Template struct {
	ID        uuid.UUID       `json:"id"`
	ProjectID *uuid.UUID      `json:"project_id"`
	Scope     string          `json:"scope"` // global or project
	Name      string          `json:"name"`
	Title     string          `json:"title"`
	Location  string          `json:"location"`
	Page      json.RawMessage `json:"page"`
	Position  int32           `json:"position"`
	Retired   bool            `json:"retired"`
}

func templateView(t sqlc.MeetingTemplate) Template {
	scope := "project"
	if t.ProjectID == nil {
		scope = "global"
	}
	return Template{ID: t.ID, ProjectID: t.ProjectID, Scope: scope, Name: t.Name, Title: t.Title, Location: t.Location,
		Page: t.Page, Position: t.Position, Retired: t.RetiredAt != nil}
}

type templateRequest struct {
	ProjectID *uuid.UUID      `json:"project_id"`
	Name      string          `json:"name" validate:"required,max=100"`
	Title     string          `json:"title" validate:"max=255"`
	Location  string          `json:"location" validate:"max=500"`
	Page      json.RawMessage `json:"page"`
	Position  *int32          `json:"position"`
}

func (r *templateRequest) Normalize() {
	r.Name, r.Title, r.Location = strings.TrimSpace(r.Name), strings.TrimSpace(r.Title), strings.TrimSpace(r.Location)
}

func normalized(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

// mayManage: the catalog administrator for the global ones, whoever manages
// the project for its own (RF-29, RF-30).
func mayManage(ctx context.Context, q *sqlc.Queries, p *authz.Principal, projectID *uuid.UUID) error {
	if projectID == nil {
		if !p.Has(authz.CatalogManage) {
			return apperr.ErrForbidden
		}
		return nil
	}
	project, err := projects.Get(ctx, q, p, *projectID, false)
	if err != nil {
		return err
	}
	if !p.CanManageProject(projects.Ref(project)) {
		return apperr.ErrForbidden
	}
	return projects.Writable(project)
}

// Choose lists what a new meeting of the project may start from (RF-34).
func (s *Service) Choose(ctx context.Context, p *authz.Principal, projectID uuid.UUID) ([]Template, error) {
	q := s.store.Queries
	if _, err := projects.Get(ctx, q, p, projectID, false); err != nil {
		return nil, err
	}
	rows, err := q.ChooseMeetingTemplates(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("listar plantillas: %w", err)
	}
	out := make([]Template, 0, len(rows))
	for _, r := range rows {
		out = append(out, templateView(r))
	}
	return out, nil
}

// Global lists every global template, retired ones too, for the catalog.
func (s *Service) Global(ctx context.Context, p *authz.Principal) ([]Template, error) {
	if !p.Has(authz.CatalogManage) {
		return nil, apperr.ErrForbidden
	}
	rows, err := s.store.GlobalMeetingTemplates(ctx)
	if err != nil {
		return nil, fmt.Errorf("listar plantillas: %w", err)
	}
	out := make([]Template, 0, len(rows))
	for _, r := range rows {
		out = append(out, templateView(r))
	}
	return out, nil
}

// templatePage validates the page of a template: the blocks of the notes but
// images and files (RF-32), which a page with no files of its own rejects.
func templatePage(raw json.RawMessage) (json.RawMessage, error) {
	doc, _, err := files.NewPage(raw)
	return doc, err
}

func (s *Service) CreateTemplate(ctx context.Context, p *authz.Principal, req templateRequest) (Template, error) {
	var out Template
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		if err := mayManage(ctx, q, p, req.ProjectID); err != nil {
			return err
		}
		page, err := templatePage(req.Page)
		if err != nil {
			return err
		}
		t, err := q.CreateMeetingTemplate(ctx, sqlc.CreateMeetingTemplateParams{
			ProjectID: req.ProjectID, Name: req.Name, NameNormalized: normalized(req.Name), Title: req.Title, Location: req.Location, Page: page,
		})
		if auth.IsUniqueViolation(err) {
			return errTemplateName // RF-33
		}
		if err != nil {
			return fmt.Errorf("crear plantilla: %w", err)
		}
		out = templateView(t)
		return recordTemplate(ctx, q, p, "created", t)
	})
	return out, err
}

func loadTemplate(ctx context.Context, q *sqlc.Queries, p *authz.Principal, id uuid.UUID) (sqlc.MeetingTemplate, error) {
	t, err := q.GetMeetingTemplateForUpdate(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, apperr.ErrNotFound
	}
	if err != nil {
		return t, fmt.Errorf("leer plantilla: %w", err)
	}
	return t, mayManage(ctx, q, p, t.ProjectID)
}

// UpdateTemplate changes a template; the meetings made from it stay as they are (RF-36).
func (s *Service) UpdateTemplate(ctx context.Context, p *authz.Principal, id uuid.UUID, req templateRequest) (Template, error) {
	var out Template
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		t, err := loadTemplate(ctx, q, p, id)
		if err != nil {
			return err
		}
		page, err := templatePage(req.Page)
		if err != nil {
			return err
		}
		position := t.Position
		if req.Position != nil {
			position = *req.Position
		}
		updated, err := q.UpdateMeetingTemplate(ctx, sqlc.UpdateMeetingTemplateParams{
			ID: id, Name: req.Name, NameNormalized: normalized(req.Name), Title: req.Title, Location: req.Location, Page: page, Position: position,
		})
		if auth.IsUniqueViolation(err) {
			return errTemplateName
		}
		if err != nil {
			return fmt.Errorf("actualizar plantilla: %w", err)
		}
		out = templateView(updated)
		return recordTemplate(ctx, q, p, "updated", updated)
	})
	return out, err
}

// SetTemplateRetired retires or restores a global template (RF-29).
func (s *Service) SetTemplateRetired(ctx context.Context, p *authz.Principal, id uuid.UUID, retired bool) (Template, error) {
	var out Template
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		t, err := loadTemplate(ctx, q, p, id)
		if err != nil {
			return err
		}
		if t.ProjectID != nil {
			return apperr.ErrValidation.WithDetail("field", "id")
		}
		updated, err := q.SetMeetingTemplateRetired(ctx, sqlc.SetMeetingTemplateRetiredParams{ID: id, Retired: retired})
		if err != nil {
			return fmt.Errorf("retirar plantilla: %w", err)
		}
		out = templateView(updated)
		action := "restored"
		if retired {
			action = "retired"
		}
		return recordTemplate(ctx, q, p, action, updated)
	})
	return out, err
}

// DeleteTemplate removes a template of a project (RF-30).
func (s *Service) DeleteTemplate(ctx context.Context, p *authz.Principal, id uuid.UUID) error {
	return s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		t, err := loadTemplate(ctx, q, p, id)
		if err != nil {
			return err
		}
		if t.ProjectID == nil {
			return apperr.ErrValidation.WithDetail("field", "id") // the global ones retire
		}
		if err := q.DeleteMeetingTemplate(ctx, id); err != nil {
			return fmt.Errorf("borrar plantilla: %w", err)
		}
		return recordTemplate(ctx, q, p, "deleted", t)
	})
}

// recordTemplate audits a change of a template, its page only as changed (RF-37).
func recordTemplate(ctx context.Context, q *sqlc.Queries, p *authz.Principal, action string, t sqlc.MeetingTemplate) error {
	return audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "meeting_template." + action, TargetType: "meeting_template",
		TargetID: t.ID.String(), After: map[string]any{"name": t.Name, "project_id": t.ProjectID, "title": t.Title, "location": t.Location, "retired": t.RetiredAt != nil}})
}
