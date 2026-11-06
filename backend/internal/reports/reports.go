// Package reports holds the AI reports of the projects (spec 020): the
// templates the administrator defines, the reports saved in each project and
// their generation through the agent service (ADR 0004).
package reports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
	"openscum/internal/llm"
	"openscum/internal/projects"
	"openscum/internal/richtext"
)

var (
	errTemplateName     = apperr.New(http.StatusConflict, "report_template_name_taken", "Ya hay una plantilla de informe con ese nombre")
	errTemplateArchived = apperr.New(http.StatusConflict, "report_template_archived", "La plantilla está archivada")
)

// Service keeps the templates and the reports. AI and Agent read the live
// settings, so a change in the system screen applies at once.
type Service struct {
	store *db.Store
	ai    func() llm.Config
	agent func() string
}

func NewService(store *db.Store, ai func() llm.Config, agent func() string) *Service {
	return &Service{store: store, ai: ai, agent: agent}
}

// Ready reports whether reports can be generated (RF-16).
func (s *Service) Ready() bool { return s.ai().Ready() && s.agent() != "" }

// Ref is a person as the reports show them.
type Ref struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// Template is a kind of report (RF-18). The prompt goes only to the
// administrator.
type Template struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Prompt      string    `json:"prompt,omitempty"`
	Archived    bool      `json:"archived"`
}

func templateView(t sqlc.ReportTemplate, withPrompt bool) Template {
	out := Template{ID: t.ID, Name: t.Name, Description: t.Description, Archived: t.ArchivedAt != nil}
	if withPrompt {
		out.Prompt = t.Prompt
	}
	return out
}

type templateRequest struct {
	Name        string `json:"name" validate:"required,max=100"`
	Description string `json:"description" validate:"max=500"`
	Prompt      string `json:"prompt" validate:"required,max=20000"`
}

func (r *templateRequest) Normalize() {
	r.Name, r.Description, r.Prompt = strings.TrimSpace(r.Name), strings.TrimSpace(r.Description), strings.TrimSpace(r.Prompt)
}

func normalized(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

// AllTemplates lists every template for the system screen.
func (s *Service) AllTemplates(ctx context.Context, p *authz.Principal) ([]Template, error) {
	if !p.Has(authz.SystemManage) {
		return nil, apperr.ErrForbidden
	}
	rows, err := s.store.ListReportTemplates(ctx)
	if err != nil {
		return nil, fmt.Errorf("listar plantillas de informe: %w", err)
	}
	out := make([]Template, 0, len(rows))
	for _, r := range rows {
		out = append(out, templateView(r, true))
	}
	return out, nil
}

// ActiveTemplates lists what a new report may use (RF-25, RF-35).
func (s *Service) ActiveTemplates(ctx context.Context) ([]Template, error) {
	rows, err := s.store.ActiveReportTemplates(ctx)
	if err != nil {
		return nil, fmt.Errorf("listar plantillas de informe: %w", err)
	}
	out := make([]Template, 0, len(rows))
	for _, r := range rows {
		out = append(out, templateView(r, false))
	}
	return out, nil
}

// recordTemplate audits a template; the prompt only as changed (RF-27).
func recordTemplate(ctx context.Context, q *sqlc.Queries, p *authz.Principal, action string, before *sqlc.ReportTemplate, after sqlc.ReportTemplate) error {
	now := map[string]any{"name": after.Name, "description": after.Description, "archived": after.ArchivedAt != nil}
	var old map[string]any
	if before != nil {
		old = map[string]any{"name": before.Name, "description": before.Description, "archived": before.ArchivedAt != nil}
		if before.Prompt != after.Prompt {
			now["prompt"] = "prompt_changed"
		}
	} else {
		now["prompt"] = "prompt_changed"
	}
	return audit.Record(ctx, q, audit.Entry{
		Actor: audit.ByUser(p), Action: "report_template." + action, TargetType: "report_template", TargetID: after.ID.String(),
		Before: old, After: now,
	})
}

func (s *Service) CreateTemplate(ctx context.Context, p *authz.Principal, req templateRequest) (Template, error) {
	if !p.Has(authz.SystemManage) {
		return Template{}, apperr.ErrForbidden
	}
	var out Template
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		t, err := q.CreateReportTemplate(ctx, sqlc.CreateReportTemplateParams{
			Name: req.Name, NameNormalized: normalized(req.Name), Description: req.Description, Prompt: req.Prompt,
		})
		if auth.IsUniqueViolation(err) {
			return errTemplateName // RF-21
		}
		if err != nil {
			return fmt.Errorf("crear plantilla de informe: %w", err)
		}
		out = templateView(t, true)
		return recordTemplate(ctx, q, p, "created", nil, t)
	})
	return out, err
}

func (s *Service) template(ctx context.Context, q *sqlc.Queries, id uuid.UUID) (sqlc.ReportTemplate, error) {
	t, err := q.GetReportTemplate(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, apperr.ErrNotFound
	}
	if err != nil {
		return t, fmt.Errorf("leer plantilla de informe: %w", err)
	}
	return t, nil
}

// UpdateTemplate changes a template; the saved reports stay as they are (RF-24).
func (s *Service) UpdateTemplate(ctx context.Context, p *authz.Principal, id uuid.UUID, req templateRequest) (Template, error) {
	if !p.Has(authz.SystemManage) {
		return Template{}, apperr.ErrForbidden
	}
	var out Template
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		before, err := s.template(ctx, q, id)
		if err != nil {
			return err
		}
		if before.ArchivedAt != nil {
			return errTemplateArchived
		}
		t, err := q.UpdateReportTemplate(ctx, sqlc.UpdateReportTemplateParams{
			ID: id, Name: req.Name, NameNormalized: normalized(req.Name), Description: req.Description, Prompt: req.Prompt,
		})
		if auth.IsUniqueViolation(err) {
			return errTemplateName
		}
		if err != nil {
			return fmt.Errorf("actualizar plantilla de informe: %w", err)
		}
		out = templateView(t, true)
		return recordTemplate(ctx, q, p, "updated", &before, t)
	})
	return out, err
}

// ArchiveTemplate stops offering a template (RF-20, RF-25).
func (s *Service) ArchiveTemplate(ctx context.Context, p *authz.Principal, id uuid.UUID) (Template, error) {
	if !p.Has(authz.SystemManage) {
		return Template{}, apperr.ErrForbidden
	}
	var out Template
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		before, err := s.template(ctx, q, id)
		if err != nil {
			return err
		}
		t, err := q.ArchiveReportTemplate(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return errTemplateArchived
		}
		if err != nil {
			return fmt.Errorf("archivar plantilla de informe: %w", err)
		}
		out = templateView(t, true)
		return recordTemplate(ctx, q, p, "archived", &before, t)
	})
	return out, err
}

// Summary is a report in the list of its project (RF-29, RF-30).
type Summary struct {
	ID           uuid.UUID  `json:"id"`
	ProjectID    uuid.UUID  `json:"project_id"`
	TemplateID   *uuid.UUID `json:"template_id"`
	TemplateName string     `json:"template_name"`
	Title        string     `json:"title"`
	Author       Ref        `json:"author"`
	UpdatedBy    Ref        `json:"updated_by"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	Version      int32      `json:"version"`
}

// Report is a whole report: its Markdown and the document to show it, built
// from the Markdown so no HTML is ever interpreted (RF-31, RF-32).
type Report struct {
	Summary
	Content string          `json:"content"`
	Doc     json.RawMessage `json:"doc"`
	Can     Can             `json:"can"`
}

// Can says what the caller may do with the reports of a project (RF-34).
type Can struct {
	Manage   bool `json:"manage"`
	Generate bool `json:"generate"`
}

// List is the reports of a project with what the caller may do.
type List struct {
	Items []Summary `json:"items"`
	Can   Can       `json:"can"`
}

// Doc renders Markdown as a clean document (RF-31, RF-32).
func Doc(markdown string) (json.RawMessage, error) {
	doc, _, err := richtext.Clean(richtext.FromMarkdown(markdown))
	return doc, err
}

func (s *Service) can(p *authz.Principal, project sqlc.Project) Can {
	manage := p.CanManageProject(projects.Ref(project))
	return Can{Manage: manage, Generate: manage && s.Ready()}
}

func (s *Service) List(ctx context.Context, p *authz.Principal, projectID uuid.UUID) (List, error) {
	q := s.store.Queries
	project, err := projects.Get(ctx, q, p, projectID, false)
	if err != nil {
		return List{}, err
	}
	rows, err := q.ListProjectReports(ctx, projectID)
	if err != nil {
		return List{}, fmt.Errorf("listar informes: %w", err)
	}
	out := List{Items: make([]Summary, 0, len(rows)), Can: s.can(p, project)}
	for _, r := range rows {
		out.Items = append(out.Items, Summary{
			ID: r.ID, ProjectID: r.ProjectID, TemplateID: r.TemplateID, TemplateName: r.TemplateName, Title: r.Title,
			Author: Ref{r.AuthorID, r.AuthorName}, UpdatedBy: Ref{r.UpdatedBy, r.UpdatedByName},
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Version: r.Version,
		})
	}
	return out, nil
}

// load reads a report and its project, checking the caller sees it.
func (s *Service) load(ctx context.Context, q *sqlc.Queries, p *authz.Principal, id uuid.UUID) (sqlc.GetProjectReportRow, sqlc.Project, error) {
	r, err := q.GetProjectReport(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, sqlc.Project{}, apperr.ErrNotFound
	}
	if err != nil {
		return r, sqlc.Project{}, fmt.Errorf("leer informe: %w", err)
	}
	project, err := projects.Get(ctx, q, p, r.ProjectID, false)
	return r, project, err
}

func (s *Service) Get(ctx context.Context, p *authz.Principal, id uuid.UUID) (Report, error) {
	r, project, err := s.load(ctx, s.store.Queries, p, id)
	if err != nil {
		return Report{}, err
	}
	doc, err := Doc(r.Content)
	if err != nil {
		return Report{}, err
	}
	return Report{
		Summary: Summary{
			ID: r.ID, ProjectID: r.ProjectID, TemplateID: r.TemplateID, TemplateName: r.TemplateName, Title: r.Title,
			Author: Ref{r.AuthorID, r.AuthorName}, UpdatedBy: Ref{r.UpdatedBy, r.UpdatedByName},
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Version: r.Version,
		},
		Content: r.Content, Doc: doc, Can: s.can(p, project),
	}, nil
}

// Origin says where the saved content came from, for the audit (RF-70).
const (
	OriginAI       = "ai"        // as the AI wrote it
	OriginAIEdited = "ai_edited" // written by the AI and retouched by hand
	OriginManual   = "manual"    // written by hand
)

type saveRequest struct {
	TemplateID *uuid.UUID `json:"template_id"`
	Title      string     `json:"title" validate:"required,max=200"`
	Content    string     `json:"content" validate:"required,max=200000"`
	Origin     string     `json:"origin" validate:"oneof=ai ai_edited manual"`
	Version    int32      `json:"version"`
}

func (r *saveRequest) Normalize() { r.Title = strings.TrimSpace(r.Title) }

func (s *Service) manage(ctx context.Context, q *sqlc.Queries, p *authz.Principal, projectID uuid.UUID) error {
	project, err := projects.Get(ctx, q, p, projectID, false)
	if err != nil {
		return err
	}
	if !p.CanManageProject(projects.Ref(project)) {
		return apperr.ErrForbidden // RF-34
	}
	return nil
}

func record(ctx context.Context, q *sqlc.Queries, p *authz.Principal, action string, id uuid.UUID, before, after any) error {
	return audit.Record(ctx, q, audit.Entry{
		Actor: audit.ByUser(p), Action: "report." + action, TargetType: "project_report", TargetID: id.String(), Before: before, After: after,
	})
}

// Create saves a new report in the project (RF-62).
func (s *Service) Create(ctx context.Context, p *authz.Principal, projectID uuid.UUID, req saveRequest) (Report, error) {
	if strings.TrimSpace(req.Content) == "" {
		return Report{}, apperr.ErrValidation.WithDetail("field", "content")
	}
	var id uuid.UUID
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		if err := s.manage(ctx, q, p, projectID); err != nil {
			return err
		}
		var templateName string
		if req.TemplateID != nil {
			t, err := s.template(ctx, q, *req.TemplateID)
			if err != nil {
				return err
			}
			templateName = t.Name
		}
		var err error
		id, err = q.CreateProjectReport(ctx, sqlc.CreateProjectReportParams{
			ProjectID: projectID, TemplateID: req.TemplateID, TemplateName: templateName, Title: req.Title, Content: req.Content, AuthorID: p.UserID,
		})
		if err != nil {
			return fmt.Errorf("crear informe: %w", err)
		}
		return record(ctx, q, p, "created", id, nil, map[string]any{
			"project_id": projectID, "title": req.Title, "template": templateName, "content": "content_changed", "origin": req.Origin,
		})
	})
	if err != nil {
		return Report{}, err
	}
	return s.Get(ctx, p, id)
}

// Update replaces the title and the content; no versions are kept (RF-63, RF-64).
func (s *Service) Update(ctx context.Context, p *authz.Principal, id uuid.UUID, req saveRequest) (Report, error) {
	if strings.TrimSpace(req.Content) == "" {
		return Report{}, apperr.ErrValidation.WithDetail("field", "content")
	}
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		r, err := q.GetProjectReportForUpdate(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("leer informe: %w", err)
		}
		if err := s.manage(ctx, q, p, r.ProjectID); err != nil {
			return err
		}
		if r.Version != req.Version {
			return apperr.VersionConflict(r.Version)
		}
		if err := q.UpdateProjectReport(ctx, sqlc.UpdateProjectReportParams{ID: id, Title: req.Title, Content: req.Content, UpdatedBy: p.UserID}); err != nil {
			return fmt.Errorf("guardar informe: %w", err)
		}
		after := map[string]any{"title": req.Title, "origin": req.Origin}
		if r.Content != req.Content {
			after["content"] = "content_changed"
		}
		return record(ctx, q, p, "updated", id, map[string]any{"title": r.Title}, after)
	})
	if err != nil {
		return Report{}, err
	}
	return s.Get(ctx, p, id)
}

// Delete removes a report (RF-68).
func (s *Service) Delete(ctx context.Context, p *authz.Principal, id uuid.UUID) error {
	return s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		r, err := q.GetProjectReportForUpdate(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("leer informe: %w", err)
		}
		if err := s.manage(ctx, q, p, r.ProjectID); err != nil {
			return err
		}
		if err := q.DeleteProjectReport(ctx, id); err != nil {
			return fmt.Errorf("borrar informe: %w", err)
		}
		return record(ctx, q, p, "deleted", id, map[string]any{"project_id": r.ProjectID, "title": r.Title}, nil)
	})
}
