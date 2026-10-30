package projects

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/authz"
	"openscum/internal/customfields"
	"openscum/internal/db/sqlc"
	"openscum/internal/files"
	"openscum/internal/workcore"
)

type Template struct {
	ID          uuid.UUID     `json:"id"`
	Identifier  string        `json:"identifier"`
	Name        string        `json:"name"`
	Team        workcore.Ref  `json:"team"`
	Methodology *workcore.Ref `json:"methodology"`
}

type fromTemplateRequest struct {
	TemplateID     uuid.UUID      `json:"template_id" validate:"required"`
	Identifier     string         `json:"identifier" validate:"required,max=10"`
	Name           string         `json:"name" validate:"required,max=255"`
	TeamID         uuid.UUID      `json:"team_id" validate:"required"`
	OwnerID        *uuid.UUID     `json:"owner_id"`
	ParentID       *uuid.UUID     `json:"parent_id"`
	StartDate      *workcore.Date `json:"start_date"`
	EndDate        *workcore.Date `json:"end_date"`
	IncludeContent bool           `json:"include_content"`
}

func (r *fromTemplateRequest) Normalize() {
	r.Identifier = strings.ToUpper(strings.TrimSpace(r.Identifier))
	r.Name = strings.TrimSpace(r.Name)
}

var (
	errNotTemplate       = apperr.New(http.StatusBadRequest, "not_a_template", "El proyecto no es una plantilla")
	errTemplateObjective = apperr.New(http.StatusConflict, "template_has_objectives", "Un proyecto ligado a objetivos no puede ser plantilla")
)

// Templates lists the templates (RF-34, RF-35).
func (s *Service) Templates(ctx context.Context) ([]Template, error) {
	rows, err := s.store.ListTemplates(ctx)
	if err != nil {
		return nil, fmt.Errorf("listar plantillas: %w", err)
	}
	out := make([]Template, 0, len(rows))
	for _, r := range rows {
		t := Template{ID: r.ID, Identifier: r.Identifier, Name: r.Name, Team: workcore.Ref{ID: r.TeamID, Name: r.TeamName}}
		if r.MethodologyID != nil && r.MethodologyName != nil {
			t.Methodology = &workcore.Ref{ID: *r.MethodologyID, Name: *r.MethodologyName}
		}
		out = append(out, t)
	}
	return out, nil
}

// SetTemplate marks a project as template or back (RF-34). A template cannot
// be linked to objectives (RF-35).
func (s *Service) SetTemplate(ctx context.Context, p *authz.Principal, id uuid.UUID, template bool) (Project, error) {
	var out Project
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		row, err := Get(ctx, q, p, id, true)
		if err != nil {
			return err
		}
		if template {
			linked, err := q.ProjectHasObjectives(ctx, id)
			if err != nil {
				return fmt.Errorf("comprobar objetivos: %w", err)
			}
			if linked {
				return errTemplateObjective
			}
		}
		updated, err := q.SetProjectTemplate(ctx, sqlc.SetProjectTemplateParams{ID: id, IsTemplate: template})
		if err != nil {
			return fmt.Errorf("marcar plantilla: %w", err)
		}
		action := "project.template_marked"
		if !template {
			action = "project.template_unmarked"
		}
		if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: action, TargetType: "project", TargetID: id.String(),
			Before: row.IsTemplate, After: template}); err != nil {
			return err
		}
		out, err = view(ctx, q, p, updated)
		return err
	})
	return out, err
}

// FromTemplate creates a project copying the configuration of a template and,
// optionally, its milestones and work items (RF-36 to RF-39). Later changes
// of the template do not reach the copy: nothing is shared.
func (s *Service) FromTemplate(ctx context.Context, p *authz.Principal, req fromTemplateRequest) (Project, error) {
	if !p.Has(authz.TemplatesUse) {
		return Project{}, apperr.ErrForbidden
	}
	var out Project
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		tpl, err := Get(ctx, q, p, req.TemplateID, false)
		if err != nil {
			return err
		}
		if !tpl.IsTemplate {
			return errNotTemplate
		}
		types, err := q.ListProjectTypeIDs(ctx, tpl.ID)
		if err != nil {
			return fmt.Errorf("tipos de la plantilla: %w", err)
		}
		fieldIDs, values, err := templateFields(ctx, q, tpl.ID)
		if err != nil {
			return err
		}
		methodology := tpl.MethodologyID
		if methodology != nil {
			if m, err := q.GetMethodology(ctx, *methodology); err != nil || m.RetiredAt != nil {
				methodology = nil
			}
		}
		// The page is set below, once its files are duplicated (RF-53).
		row, err := create(ctx, q, p, createRequest{
			Identifier: req.Identifier, Name: req.Name, TeamID: req.TeamID,
			OwnerID: req.OwnerID, MethodologyID: methodology, ParentID: req.ParentID,
			StartDate: req.StartDate, EndDate: req.EndDate, HealthExplanation: "",
			TypeIDs: append([]uuid.UUID{}, types...), CustomFieldIDs: fieldIDs, CustomValues: values,
		}, &tpl.ID)
		if err != nil {
			return err
		}
		if tpl.Description != nil {
			err := s.copyPage(ctx, q, p, page("project", tpl.ID, tpl.ID), page("project", row.ID, row.ID), tpl.Description,
				func(doc json.RawMessage) error {
					return q.SetProjectDescription(ctx, sqlc.SetProjectDescriptionParams{ID: row.ID, Description: doc, DescriptionText: tpl.DescriptionText})
				})
			if err != nil {
				return err
			}
		}
		if req.IncludeContent {
			shift := 0
			if tpl.StartDate != nil && req.StartDate != nil {
				shift = int(req.StartDate.Sub(*tpl.StartDate).Hours() / 24)
			}
			if err := s.copyContent(ctx, q, p, tpl.ID, row.ID, shift); err != nil {
				return err
			}
		}
		if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "project.created_from_template", TargetType: "project",
			TargetID: row.ID.String(), After: map[string]any{"template_id": tpl.ID, "include_content": req.IncludeContent}}); err != nil {
			return err
		}
		out, err = view(ctx, q, p, row)
		return err
	})
	return out, err
}

// templateFields returns the active fields enabled in a template and its
// values for them.
func templateFields(ctx context.Context, q *sqlc.Queries, template uuid.UUID) ([]uuid.UUID, customfields.Values, error) {
	defs, err := customfields.Load(ctx, q)
	if err != nil {
		return nil, nil, err
	}
	enabled, err := customfields.EnabledFields(ctx, q, template)
	if err != nil {
		return nil, nil, err
	}
	stored, err := q.ListProjectCustomValues(ctx, template)
	if err != nil {
		return nil, nil, fmt.Errorf("valores de la plantilla: %w", err)
	}
	ids := []uuid.UUID{}
	values := customfields.Values{}
	for _, f := range defs {
		if !enabled[f.ID] || f.Retired() {
			continue
		}
		ids = append(ids, f.ID)
		for _, v := range stored {
			if v.FieldID == f.ID && f.Entity == customfields.EntityProject {
				values[f.ID] = v.Value
			}
		}
	}
	return ids, values, nil
}

func shiftDate(t *time.Time, days int) *time.Time {
	if t == nil {
		return nil
	}
	d := t.AddDate(0, 0, days)
	return &d
}

// copyContent copies milestones, work items (hierarchy kept, new numbers,
// default status, no assignee), internal dependencies, milestone links and
// work item values from a template (RF-37).
func (s *Service) copyContent(ctx context.Context, q *sqlc.Queries, p *authz.Principal, from, to uuid.UUID, shift int) error {
	status, err := q.GetDefaultStatus(ctx)
	if err != nil {
		return fmt.Errorf("estado por defecto: %w", err)
	}
	// Phases are matched by definition: both projects come from the same methodology.
	phases := map[uuid.UUID]uuid.UUID{}
	oldPhases, err := q.ListProjectPhases(ctx, from)
	if err != nil {
		return fmt.Errorf("fases de la plantilla: %w", err)
	}
	newPhases, err := q.ListProjectPhases(ctx, to)
	if err != nil {
		return fmt.Errorf("fases del proyecto: %w", err)
	}
	for _, o := range oldPhases {
		for _, n := range newPhases {
			if o.DefinitionID == n.DefinitionID {
				phases[o.ID] = n.ID
			}
		}
	}

	milestones := map[uuid.UUID]uuid.UUID{}
	ms, err := q.ListProjectMilestones(ctx, from)
	if err != nil {
		return fmt.Errorf("hitos de la plantilla: %w", err)
	}
	for _, m := range ms {
		var phase *uuid.UUID
		if m.PhaseID != nil {
			if np, ok := phases[*m.PhaseID]; ok {
				phase = &np
			}
		}
		id, err := q.CopyMilestone(ctx, sqlc.CopyMilestoneParams{
			ProjectID: to, PhaseID: phase, Name: m.Name, Description: m.Description, DescriptionText: m.DescriptionText,
			DueDate: shiftDate(m.DueDate, shift), Position: m.Position,
		})
		if err != nil {
			return fmt.Errorf("copiar hito: %w", err)
		}
		milestones[m.ID] = id
		err = s.copyPage(ctx, q, p, page("milestone", m.ID, from), page("milestone", id, to), m.Description,
			func(doc json.RawMessage) error {
				return q.SetMilestoneDescription(ctx, sqlc.SetMilestoneDescriptionParams{ID: id, Description: doc, DescriptionText: m.DescriptionText})
			})
		if err != nil {
			return err
		}
	}

	items := map[uuid.UUID]uuid.UUID{}
	rows, err := q.ListProjectWorkItemsForCopy(ctx, from) // parents first
	if err != nil {
		return fmt.Errorf("work items de la plantilla: %w", err)
	}
	for i, w := range rows {
		var parent *uuid.UUID
		if w.ParentID != nil {
			np := items[*w.ParentID]
			parent = &np
		}
		id, err := q.CopyWorkItem(ctx, sqlc.CopyWorkItemParams{
			ProjectID: to, TypeID: w.TypeID, StatusID: status.ID, PriorityID: w.PriorityID, ParentID: parent,
			SequenceNumber: int32(i + 1), Title: w.Title, Description: w.Description, DescriptionText: w.DescriptionText,
			AuthorID: p.UserID, StartDate: shiftDate(w.StartDate, shift), DueDate: shiftDate(w.DueDate, shift),
			EstimatedHours: w.EstimatedHours, RemainingHours: w.EstimatedHours, Position: w.Position, Depth: w.Depth,
			Weight: w.Weight, SizeID: w.SizeID, // RF-157; spec 006, RF-11
		})
		if err != nil {
			return fmt.Errorf("copiar work item: %w", err)
		}
		items[w.ID] = id
		err = s.copyPage(ctx, q, p, page("work_item", w.ID, from), page("work_item", id, to), w.Description,
			func(doc json.RawMessage) error {
				return q.SetWorkItemDescription(ctx, sqlc.SetWorkItemDescriptionParams{ID: id, Description: doc, DescriptionText: w.DescriptionText})
			})
		if err != nil {
			return err
		}
	}
	if err := q.SetWorkItemCounter(ctx, sqlc.SetWorkItemCounterParams{ID: to, WiCounter: int32(len(rows))}); err != nil {
		return fmt.Errorf("contador de work items: %w", err)
	}

	deps, err := q.ListProjectDependenciesForCopy(ctx, from)
	if err != nil {
		return fmt.Errorf("dependencias de la plantilla: %w", err)
	}
	for _, d := range deps {
		if err := q.CopyDependency(ctx, sqlc.CopyDependencyParams{
			FromID: items[d.FromID], ToID: items[d.ToID], Type: d.Type, LagDays: d.LagDays, CreatedBy: p.UserID,
		}); err != nil {
			return fmt.Errorf("copiar dependencia: %w", err)
		}
	}
	links, err := q.ListProjectMilestoneLinksForCopy(ctx, from)
	if err != nil {
		return fmt.Errorf("vínculos de la plantilla: %w", err)
	}
	for _, l := range links {
		if err := q.CopyMilestoneLink(ctx, sqlc.CopyMilestoneLinkParams{MilestoneID: milestones[l.MilestoneID], WorkItemID: items[l.WorkItemID], ProjectID: to}); err != nil {
			return fmt.Errorf("copiar vínculo: %w", err)
		}
	}
	values, err := q.ListProjectWorkItemValuesForCopy(ctx, from)
	if err != nil {
		return fmt.Errorf("valores de la plantilla: %w", err)
	}
	for _, v := range values {
		if err := q.CopyWorkItemValue(ctx, sqlc.CopyWorkItemValueParams{WorkItemID: items[v.WorkItemID], FieldID: v.FieldID, Value: v.Value}); err != nil {
			return fmt.Errorf("copiar valor: %w", err)
		}
	}
	if err := q.RecomputeWorkItemAggregates(ctx, to); err != nil {
		return fmt.Errorf("recalcular derivados: %w", err)
	}
	return nil
}

// page names an owner of a page: its kind, its id and its project.
func page(kind string, id, project uuid.UUID) files.Owner {
	return files.Owner{Kind: kind, ID: id, ProjectID: project}
}

// copyPage duplicates the files of from's page for to (RF-53) and, when there
// are any, stores the copied page pointing at the new files with set. A uuid
// is unique, so replacing each quoted old id only touches its fileId.
func (s *Service) copyPage(ctx context.Context, q *sqlc.Queries, p *authz.Principal, from, to files.Owner,
	doc json.RawMessage, set func(json.RawMessage) error) error {
	ids, err := s.files.Copy(ctx, q, from, to, p.UserID)
	if err != nil {
		return err
	}
	if len(ids) == 0 && to.Kind != "project" {
		return nil // the copy already holds the same page
	}
	out := string(doc)
	for old, copied := range ids {
		out = strings.ReplaceAll(out, `"`+old.String()+`"`, `"`+copied.String()+`"`)
	}
	return set(json.RawMessage(out))
}
