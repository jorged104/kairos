// Package milestones manages the milestones of a project, the work items that
// count for them (inherited by composition) and their progress (RF-44 to
// RF-60, RF-119).
package milestones

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
	"openscum/internal/authz"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
	"openscum/internal/events"
	"openscum/internal/projects"
	"openscum/internal/richtext"
	"openscum/internal/workcore"
	"openscum/internal/workitems"
)

const (
	StatusOpen      = "abierto"
	StatusCompleted = "cumplido"
	StatusCancelled = "cancelado"
)

type Progress struct {
	Total          int32   `json:"total"`
	Closed         int32   `json:"closed"`
	Open           int32   `json:"open"`
	Percent        int     `json:"percent"`
	EstimatedHours float64 `json:"estimated_hours"`
	RemainingHours float64 `json:"remaining_hours"`
}

type Milestone struct {
	ID          uuid.UUID            `json:"id"`
	Project     workitems.ProjectRef `json:"project"`
	Phase       *workcore.Ref        `json:"phase"`
	Name        string               `json:"name"`
	Description json.RawMessage      `json:"description"`
	DueDate     *workcore.Date       `json:"due_date"`
	CompletedOn *workcore.Date       `json:"completed_on"`
	Status      string               `json:"status"`
	Overdue     bool                 `json:"overdue"`
	Position    int32                `json:"position"`
	Version     int32                `json:"version"`
	Progress    Progress             `json:"progress"`
	Can         struct {
		Manage bool `json:"manage"`
	} `json:"can"`
}

// Item is an effective work item of a milestone.
type Item struct {
	workitems.Node
	Inherited bool `json:"inherited"`
}

// Effective is a milestone of a work item, direct or inherited (RF-53).
type Effective struct {
	ID        uuid.UUID      `json:"id"`
	Name      string         `json:"name"`
	Status    string         `json:"status"`
	DueDate   *workcore.Date `json:"due_date"`
	Inherited bool           `json:"inherited"`
}

type milestoneRequest struct {
	Version     int32           `json:"version"`
	Name        string          `json:"name" validate:"required,max=255"`
	Description json.RawMessage `json:"description"`
	DueDate     *workcore.Date  `json:"due_date"`
	PhaseID     *uuid.UUID      `json:"phase_id"`
	Position    int32           `json:"position"`
}

func (r *milestoneRequest) Normalize() { r.Name = strings.TrimSpace(r.Name) }

type statusRequest struct {
	Version     int32          `json:"version" validate:"required,min=1"`
	CompletedOn *workcore.Date `json:"completed_on"`
}

type linkRequest struct {
	WorkItemID uuid.UUID `json:"work_item_id" validate:"required"`
}

var (
	errInUse        = apperr.New(http.StatusConflict, "milestone_in_use", "El hito tiene work items u objetivos ligados; cancélalo en su lugar")
	errCrossProject = apperr.New(http.StatusBadRequest, "cross_project", "El elemento pertenece a otro proyecto")
	errStatus       = apperr.New(http.StatusConflict, "invalid_milestone_status", "El hito no está en un estado que permita esa acción")
)

type Service struct {
	store *db.Store
	now   func() time.Time
}

func NewService(store *db.Store) *Service { return &Service{store: store, now: time.Now} }

// load returns a milestone and its project if the caller can see them.
func load(ctx context.Context, q *sqlc.Queries, p *authz.Principal, id uuid.UUID, forUpdate bool) (sqlc.Milestone, sqlc.Project, error) {
	get := q.GetMilestone
	if forUpdate {
		get = q.GetMilestoneForUpdate
	}
	m, err := get(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, sqlc.Project{}, apperr.ErrNotFound
	}
	if err != nil {
		return m, sqlc.Project{}, fmt.Errorf("leer hito: %w", err)
	}
	project, err := projects.Get(ctx, q, p, m.ProjectID, false)
	return m, project, err
}

// manageable loads a milestone whose project the caller manages and can write.
func manageable(ctx context.Context, q *sqlc.Queries, p *authz.Principal, id uuid.UUID) (sqlc.Milestone, sqlc.Project, error) {
	m, project, err := load(ctx, q, p, id, true)
	if err != nil {
		return m, project, err
	}
	if !p.CanManageProject(projects.Ref(project)) {
		return m, project, apperr.ErrForbidden
	}
	return m, project, projects.Writable(project)
}

func checkPhase(ctx context.Context, q *sqlc.Queries, project uuid.UUID, phaseID *uuid.UUID) error {
	if phaseID == nil {
		return nil
	}
	ph, err := q.GetProjectPhase(ctx, *phaseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.ErrValidation.WithDetail("field", "phase_id")
	}
	if err != nil {
		return fmt.Errorf("leer fase: %w", err)
	}
	if ph.ProjectID != project {
		return errCrossProject.WithDetail("field", "phase_id") // RF-58
	}
	return nil
}

func (s *Service) Create(ctx context.Context, p *authz.Principal, projectID uuid.UUID, req milestoneRequest) (Milestone, error) {
	var out Milestone
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		project, err := projects.Get(ctx, q, p, projectID, true)
		if err != nil {
			return err
		}
		if !p.CanManageProject(projects.Ref(project)) {
			return apperr.ErrForbidden
		}
		if err := projects.Writable(project); err != nil {
			return err
		}
		if err := checkPhase(ctx, q, projectID, req.PhaseID); err != nil {
			return err
		}
		desc, text, err := richtext.Clean(req.Description)
		if err != nil {
			return err
		}
		m, err := q.CreateMilestone(ctx, sqlc.CreateMilestoneParams{
			ProjectID: projectID, PhaseID: req.PhaseID, Name: req.Name, Description: desc, DescriptionText: text,
			DueDate: workcore.ToTime(req.DueDate), Position: req.Position,
		})
		if err != nil {
			return fmt.Errorf("crear hito: %w", err)
		}
		if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "milestone.created", TargetType: "milestone",
			TargetID: m.ID.String(), After: auditView(m)}); err != nil {
			return err
		}
		out, err = s.view(ctx, q, p, m, project)
		return err
	})
	return out, err
}

func (s *Service) Get(ctx context.Context, p *authz.Principal, id uuid.UUID) (Milestone, error) {
	m, project, err := load(ctx, s.store.Queries, p, id, false)
	if err != nil {
		return Milestone{}, err
	}
	return s.view(ctx, s.store.Queries, p, m, project)
}

// List returns the milestones of a project with their progress in two queries.
func (s *Service) List(ctx context.Context, p *authz.Principal, projectID uuid.UUID) ([]Milestone, error) {
	project, err := projects.Get(ctx, s.store.Queries, p, projectID, false)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.ListProjectMilestones(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("listar hitos: %w", err)
	}
	progress, err := s.progress(ctx, s.store.Queries, sqlc.MilestoneProgressParams{ProjectID: &projectID})
	if err != nil {
		return nil, err
	}
	out := make([]Milestone, 0, len(rows))
	for _, r := range rows {
		m := s.base(p, sqlc.Milestone{
			ID: r.ID, ProjectID: r.ProjectID, PhaseID: r.PhaseID, Name: r.Name, Description: r.Description,
			DueDate: r.DueDate, CompletedOn: r.CompletedOn, Status: r.Status, Position: r.Position, Version: r.Version,
		}, project)
		if r.PhaseRefID != nil && r.PhaseName != nil {
			m.Phase = &workcore.Ref{ID: *r.PhaseRefID, Name: *r.PhaseName}
		}
		m.Progress = progress[r.ID]
		out = append(out, m)
	}
	return out, nil
}

func (s *Service) Update(ctx context.Context, p *authz.Principal, id uuid.UUID, req milestoneRequest) (Milestone, error) {
	var out Milestone
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		m, project, err := manageable(ctx, q, p, id)
		if err != nil {
			return err
		}
		if req.Version != m.Version {
			return apperr.VersionConflict(m.Version)
		}
		if err := checkPhase(ctx, q, project.ID, req.PhaseID); err != nil {
			return err
		}
		desc, text, err := richtext.Clean(req.Description)
		if err != nil {
			return err
		}
		updated, err := q.UpdateMilestone(ctx, sqlc.UpdateMilestoneParams{
			ID: id, Version: req.Version, PhaseID: req.PhaseID, Name: req.Name, Description: desc, DescriptionText: text,
			DueDate: workcore.ToTime(req.DueDate), Position: req.Position,
		})
		if err != nil {
			return fmt.Errorf("actualizar hito: %w", err)
		}
		after := auditView(updated)
		after["description_changed"] = string(m.Description) != string(desc)
		if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "milestone.updated", TargetType: "milestone",
			TargetID: id.String(), Before: auditView(m), After: after}); err != nil {
			return err
		}
		out, err = s.view(ctx, q, p, updated, project)
		return err
	})
	return out, err
}

// SetStatus completes, reopens or cancels a milestone (RF-45 to RF-49) and
// emits milestone.completed / milestone.reopened (RF-119).
func (s *Service) SetStatus(ctx context.Context, p *authz.Principal, id uuid.UUID, action string, req statusRequest) (Milestone, error) {
	var out Milestone
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		m, project, err := manageable(ctx, q, p, id)
		if err != nil {
			return err
		}
		if req.Version != m.Version {
			return apperr.VersionConflict(m.Version)
		}
		var status string
		var completedOn *time.Time
		event := ""
		switch {
		case action == "complete" && m.Status == StatusOpen:
			status = StatusCompleted
			day := time.Date(s.now().Year(), s.now().Month(), s.now().Day(), 0, 0, 0, 0, time.UTC)
			if req.CompletedOn != nil {
				day = req.CompletedOn.Time
			}
			completedOn, event = &day, events.MilestoneCompleted
		case action == "reopen" && m.Status != StatusOpen:
			status = StatusOpen
			if m.Status == StatusCompleted {
				event = events.MilestoneReopened
			}
		case action == "cancel" && m.Status == StatusOpen:
			status = StatusCancelled
		default:
			return errStatus
		}
		updated, err := q.SetMilestoneStatus(ctx, sqlc.SetMilestoneStatusParams{ID: id, Version: req.Version, Status: status, CompletedOn: completedOn})
		if err != nil {
			return fmt.Errorf("cambiar estado del hito: %w", err)
		}
		if event != "" && !project.IsTemplate {
			actor := p.UserID
			if err := events.Emit(ctx, q, events.New{
				Type: event, ActorID: &actor, ProjectID: project.ID, SubjectType: "milestone", SubjectID: id,
				Payload: map[string]any{"name": m.Name, "completed_on": workcore.FromTime(completedOn)},
			}); err != nil {
				return err
			}
		}
		if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "milestone." + action, TargetType: "milestone",
			TargetID: id.String(), Before: m.Status, After: status}); err != nil {
			return err
		}
		out, err = s.view(ctx, q, p, updated, project)
		return err
	})
	return out, err
}

// Delete removes a milestone with nothing linked (RF-59, RF-60).
func (s *Service) Delete(ctx context.Context, p *authz.Principal, id uuid.UUID) error {
	return s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		m, _, err := manageable(ctx, q, p, id)
		if err != nil {
			return err
		}
		used, err := q.MilestoneInUse(ctx, id)
		if err != nil {
			return fmt.Errorf("comprobar uso del hito: %w", err)
		}
		if used {
			return errInUse
		}
		if err := q.DeleteMilestone(ctx, id); err != nil {
			return fmt.Errorf("borrar hito: %w", err)
		}
		return audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "milestone.deleted", TargetType: "milestone",
			TargetID: id.String(), Before: auditView(m)})
	})
}

// Link makes a work item count for a milestone (RF-50, RF-51). It needs the
// permission to manage the milestone (RF-127).
func (s *Service) Link(ctx context.Context, p *authz.Principal, id, workItemID uuid.UUID, linked bool) error {
	return s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		_, project, err := manageable(ctx, q, p, id)
		if err != nil {
			return err
		}
		wi, err := q.GetWorkItem(ctx, workItemID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrValidation.WithDetail("field", "work_item_id")
		}
		if err != nil {
			return fmt.Errorf("leer work item: %w", err)
		}
		if wi.ProjectID != project.ID {
			return errCrossProject.WithDetail("field", "work_item_id")
		}
		action := "milestone.work_item_linked"
		if linked {
			err = q.LinkMilestoneWorkItem(ctx, sqlc.LinkMilestoneWorkItemParams{MilestoneID: id, WorkItemID: workItemID, ProjectID: project.ID})
		} else {
			action = "milestone.work_item_unlinked"
			var n int64
			if n, err = q.UnlinkMilestoneWorkItem(ctx, sqlc.UnlinkMilestoneWorkItemParams{MilestoneID: id, WorkItemID: workItemID}); err == nil && n == 0 {
				return apperr.ErrNotFound
			}
		}
		if err != nil {
			return fmt.Errorf("ligar work item: %w", err)
		}
		return audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: action, TargetType: "milestone",
			TargetID: id.String(), After: map[string]any{"work_item_id": workItemID}})
	})
}

// WorkItems lists the effective work items of a milestone (RF-52).
func (s *Service) WorkItems(ctx context.Context, p *authz.Principal, id uuid.UUID) ([]Item, error) {
	_, project, err := load(ctx, s.store.Queries, p, id, false)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.MilestoneEffectiveWorkItems(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("work items del hito: %w", err)
	}
	out := make([]Item, 0, len(rows))
	for _, r := range rows {
		out = append(out, Item{
			Node: workitems.NodeFrom(project.Identifier, sqlc.WorkItemDescendantsRow{
				ID: r.ID, ParentID: r.ParentID, SequenceNumber: r.SequenceNumber, Title: r.Title, TypeID: r.TypeID,
				StatusID: r.StatusID, PriorityID: r.PriorityID, AssigneeID: r.AssigneeID, AssigneeName: r.AssigneeName,
				DerivedDoneRatio: r.DerivedDoneRatio, AggEstimated: r.AggEstimated, AggRemaining: r.AggRemaining,
				Depth: r.Depth, Position: r.Position, DueDate: r.DueDate,
			}),
			Inherited: !r.Direct,
		})
	}
	return out, nil
}

// OfWorkItem lists the effective milestones of a work item (RF-53).
func (s *Service) OfWorkItem(ctx context.Context, p *authz.Principal, workItemID uuid.UUID) ([]Effective, error) {
	if _, _, err := workitems.Load(ctx, s.store.Queries, p, workItemID, false); err != nil {
		return nil, err
	}
	rows, err := s.store.WorkItemEffectiveMilestones(ctx, workItemID)
	if err != nil {
		return nil, fmt.Errorf("hitos del work item: %w", err)
	}
	out := make([]Effective, 0, len(rows))
	for _, r := range rows {
		out = append(out, Effective{ID: r.ID, Name: r.Name, Status: r.Status, DueDate: workcore.FromTime(r.DueDate), Inherited: !r.Direct})
	}
	return out, nil
}

func (s *Service) progress(ctx context.Context, q *sqlc.Queries, arg sqlc.MilestoneProgressParams) (map[uuid.UUID]Progress, error) {
	rows, err := q.MilestoneProgress(ctx, arg)
	if err != nil {
		return nil, fmt.Errorf("avance de hitos: %w", err)
	}
	out := make(map[uuid.UUID]Progress, len(rows))
	for _, r := range rows {
		pr := Progress{Total: r.Total, Closed: r.Closed, Open: r.Total - r.Closed, EstimatedHours: r.EstimatedHours, RemainingHours: r.RemainingHours}
		if r.Total > 0 {
			pr.Percent = int(float64(r.Closed) * 100 / float64(r.Total))
		}
		out[r.MilestoneID] = pr
	}
	return out, nil
}

func (s *Service) base(p *authz.Principal, m sqlc.Milestone, project sqlc.Project) Milestone {
	out := Milestone{
		ID: m.ID, Project: workitems.ProjectRef{ID: project.ID, Identifier: project.Identifier, Name: project.Name},
		Name: m.Name, Description: m.Description, DueDate: workcore.FromTime(m.DueDate),
		CompletedOn: workcore.FromTime(m.CompletedOn), Status: m.Status, Position: m.Position, Version: m.Version,
	}
	today := time.Date(s.now().Year(), s.now().Month(), s.now().Day(), 0, 0, 0, 0, time.UTC)
	out.Overdue = m.Status == StatusOpen && m.DueDate != nil && m.DueDate.Before(today) // RF-48
	out.Can.Manage = p.CanManageProject(projects.Ref(project)) && projects.Writable(project) == nil
	return out
}

func (s *Service) view(ctx context.Context, q *sqlc.Queries, p *authz.Principal, m sqlc.Milestone, project sqlc.Project) (Milestone, error) {
	out := s.base(p, m, project)
	if m.PhaseID != nil {
		ph, err := q.GetProjectPhase(ctx, *m.PhaseID)
		if err != nil {
			return out, fmt.Errorf("leer fase: %w", err)
		}
		out.Phase = &workcore.Ref{ID: ph.ID, Name: ph.Name}
	}
	progress, err := s.progress(ctx, q, sqlc.MilestoneProgressParams{MilestoneID: &m.ID})
	if err != nil {
		return out, err
	}
	out.Progress = progress[m.ID]
	return out, nil
}

func auditView(m sqlc.Milestone) map[string]any {
	return map[string]any{
		"name": m.Name, "phase_id": m.PhaseID, "due_date": workcore.FromTime(m.DueDate),
		"status": m.Status, "completed_on": workcore.FromTime(m.CompletedOn), "position": m.Position,
	}
}
