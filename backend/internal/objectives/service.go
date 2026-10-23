package objectives

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/authz"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
	"openscum/internal/projects"
	"openscum/internal/richtext"
	"openscum/internal/workcore"
	"openscum/internal/workitems"
)

type Service struct {
	store *db.Store
}

func NewService(store *db.Store) *Service { return &Service{store: store} }

// scopeTeam is the team that decides the lead scope of an objective: its own
// (equipo) or its project's (proyecto); nil for organizacion.
func scopeTeam(ctx context.Context, q *sqlc.Queries, o sqlc.Objective) (*uuid.UUID, error) {
	if o.ScopeTeamID != nil {
		return o.ScopeTeamID, nil
	}
	if o.ScopeProjectID != nil {
		p, err := q.GetProject(ctx, *o.ScopeProjectID)
		if err != nil {
			return nil, fmt.Errorf("leer proyecto del objetivo: %w", err)
		}
		return &p.TeamID, nil
	}
	return nil, nil
}

func ref(ctx context.Context, q *sqlc.Queries, o sqlc.Objective) (authz.ObjectiveRef, error) {
	team, err := scopeTeam(ctx, q, o)
	return authz.ObjectiveRef{ScopeTeamID: team, OwnerID: o.OwnerID}, err
}

func load(ctx context.Context, q *sqlc.Queries, id uuid.UUID, forUpdate bool) (sqlc.Objective, error) {
	get := q.GetObjective
	if forUpdate {
		get = q.GetObjectiveForUpdate
	}
	o, err := get(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return o, apperr.ErrNotFound
	}
	if err != nil {
		return o, fmt.Errorf("leer objetivo: %w", err)
	}
	return o, nil
}

// manageable loads an objective the caller manages; closed or cancelled
// objectives are read-only.
func manageable(ctx context.Context, q *sqlc.Queries, p *authz.Principal, id uuid.UUID) (sqlc.Objective, error) {
	o, err := load(ctx, q, id, true)
	if err != nil {
		return o, err
	}
	r, err := ref(ctx, q, o)
	if err != nil {
		return o, err
	}
	if !p.CanManageObjective(r) {
		return o, apperr.ErrForbidden
	}
	if o.Status == StatusClosed || o.Status == StatusCancelled {
		return o, errReadOnly
	}
	return o, nil
}

// Create saves an objective in draft (RF-97 to RF-99).
func (s *Service) Create(ctx context.Context, p *authz.Principal, req createRequest) (Objective, error) {
	if req.PeriodEnd.Before(req.PeriodStart.Time) {
		return Objective{}, errDateRange
	}
	var out Objective
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		var team *uuid.UUID
		var project *sqlc.Project
		switch req.Scope {
		case ScopeOrganization:
			if req.ScopeTeamID != nil || req.ScopeProjectID != nil {
				return apperr.ErrValidation.WithDetail("field", "scope")
			}
		case ScopeTeam:
			if req.ScopeTeamID == nil || req.ScopeProjectID != nil {
				return apperr.ErrValidation.WithDetail("field", "scope_team_id")
			}
			t, err := q.GetTeam(ctx, *req.ScopeTeamID)
			if errors.Is(err, pgx.ErrNoRows) || (err == nil && t.ArchivedAt != nil) {
				return apperr.ErrValidation.WithDetail("field", "scope_team_id")
			}
			if err != nil {
				return fmt.Errorf("leer equipo: %w", err)
			}
			team = req.ScopeTeamID
		case ScopeProject:
			if req.ScopeProjectID == nil || req.ScopeTeamID != nil {
				return apperr.ErrValidation.WithDetail("field", "scope_project_id")
			}
			pr, err := projects.Get(ctx, q, p, *req.ScopeProjectID, false)
			if errors.Is(err, apperr.ErrNotFound) {
				return apperr.ErrValidation.WithDetail("field", "scope_project_id")
			}
			if err != nil {
				return err
			}
			if pr.IsTemplate {
				return errTemplate
			}
			project, team = &pr, &pr.TeamID
		}
		if !p.CanCreateObjective(team) {
			return apperr.ErrForbidden
		}
		owner := p.UserID
		if req.OwnerID != nil {
			owner = *req.OwnerID
		}
		if err := workcore.CheckWorker(ctx, q, owner, errInvalidOwner); err != nil {
			return err
		}
		desc, text, err := richtext.Clean(req.Description)
		if err != nil {
			return err
		}
		o, err := q.CreateObjective(ctx, sqlc.CreateObjectiveParams{
			Title: req.Title, Description: desc, DescriptionText: text, OwnerID: owner, Scope: req.Scope,
			ScopeTeamID: req.ScopeTeamID, ScopeProjectID: req.ScopeProjectID,
			PeriodStart: req.PeriodStart.Time, PeriodEnd: req.PeriodEnd.Time,
		})
		if err != nil {
			return fmt.Errorf("crear objetivo: %w", err)
		}
		if project != nil {
			if err := q.LinkObjectiveProject(ctx, sqlc.LinkObjectiveProjectParams{ObjectiveID: o.ID, ProjectID: project.ID}); err != nil {
				return fmt.Errorf("ligar proyecto: %w", err)
			}
		}
		if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "objective.created", TargetType: "objective",
			TargetID: o.ID.String(), After: auditView(o)}); err != nil {
			return err
		}
		out, err = view(ctx, q, p, o)
		return err
	})
	return out, err
}

func (s *Service) Get(ctx context.Context, p *authz.Principal, id uuid.UUID) (Objective, error) {
	o, err := load(ctx, s.store.Queries, id, false)
	if err != nil {
		return Objective{}, err
	}
	return view(ctx, s.store.Queries, p, o)
}

func (s *Service) Update(ctx context.Context, p *authz.Principal, id uuid.UUID, req updateRequest) (Objective, error) {
	if req.PeriodEnd.Before(req.PeriodStart.Time) {
		return Objective{}, errDateRange
	}
	var out Objective
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		o, err := manageable(ctx, q, p, id)
		if err != nil {
			return err
		}
		if req.Version != o.Version {
			return apperr.VersionConflict(o.Version)
		}
		if req.OwnerID != o.OwnerID {
			if err := workcore.CheckWorker(ctx, q, req.OwnerID, errInvalidOwner); err != nil {
				return err
			}
		}
		desc, text, err := richtext.Clean(req.Description)
		if err != nil {
			return err
		}
		updated, err := q.UpdateObjective(ctx, sqlc.UpdateObjectiveParams{
			ID: id, Version: req.Version, Title: req.Title, Description: desc, DescriptionText: text,
			OwnerID: req.OwnerID, PeriodStart: req.PeriodStart.Time, PeriodEnd: req.PeriodEnd.Time,
		})
		if err != nil {
			return fmt.Errorf("actualizar objetivo: %w", err)
		}
		after := auditView(updated)
		after["description_changed"] = string(o.Description) != string(desc)
		if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "objective.updated", TargetType: "objective",
			TargetID: id.String(), Before: auditView(o), After: after}); err != nil {
			return err
		}
		out, err = view(ctx, q, p, updated)
		return err
	})
	return out, err
}

// statusMoves lists the allowed status changes per action (RF-99, RF-100).
var statusMoves = map[string]struct {
	from []string
	to   string
}{
	"activate": {[]string{StatusDraft}, StatusActive},
	"close":    {[]string{StatusActive}, StatusClosed},
	"cancel":   {[]string{StatusDraft, StatusActive}, StatusCancelled},
	"reopen":   {[]string{StatusClosed, StatusCancelled}, StatusActive},
}

func (s *Service) SetStatus(ctx context.Context, p *authz.Principal, id uuid.UUID, action string, version int32) (Objective, error) {
	move, ok := statusMoves[action]
	if !ok {
		return Objective{}, apperr.ErrNotFound
	}
	var out Objective
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		o, err := load(ctx, q, id, true)
		if err != nil {
			return err
		}
		r, err := ref(ctx, q, o)
		if err != nil {
			return err
		}
		if !p.CanManageObjective(r) {
			return apperr.ErrForbidden
		}
		if version != o.Version {
			return apperr.VersionConflict(o.Version)
		}
		valid := false
		for _, f := range move.from {
			valid = valid || o.Status == f
		}
		if !valid {
			return errStatus
		}
		if move.to == StatusActive {
			n, err := q.CountKeyResults(ctx, id)
			if err != nil {
				return fmt.Errorf("contar resultados clave: %w", err)
			}
			if n == 0 {
				return errNoKeyResults
			}
		}
		updated, err := q.SetObjectiveStatus(ctx, sqlc.SetObjectiveStatusParams{ID: id, Version: version, Status: move.to})
		if err != nil {
			return fmt.Errorf("cambiar estado del objetivo: %w", err)
		}
		if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "objective." + action, TargetType: "objective",
			TargetID: id.String(), Before: o.Status, After: move.to}); err != nil {
			return err
		}
		out, err = view(ctx, q, p, updated)
		return err
	})
	return out, err
}

// Key results (RF-101 to RF-103, RF-111) ------------------------------------------

func (s *Service) CreateKeyResult(ctx context.Context, p *authz.Principal, objectiveID uuid.UUID, req keyResultRequest) (Objective, error) {
	if req.StartValue == req.TargetValue {
		return Objective{}, errTargetEqual
	}
	return s.change(ctx, p, objectiveID, "objective.key_result_created", func(q *sqlc.Queries, o sqlc.Objective) error {
		current := req.StartValue
		if req.CalcMode == ModeManual && req.CurrentValue != nil {
			current = *req.CurrentValue
		} else if req.CalcMode == ModeDerived && req.CurrentValue != nil {
			return errDerived
		}
		_, err := q.CreateKeyResult(ctx, sqlc.CreateKeyResultParams{
			ObjectiveID: o.ID, Title: req.Title, Unit: req.Unit, StartValue: req.StartValue, TargetValue: req.TargetValue,
			CurrentValue: current, CalcMode: req.CalcMode, Position: req.Position,
		})
		return err
	})
}

func (s *Service) UpdateKeyResult(ctx context.Context, p *authz.Principal, objectiveID, id uuid.UUID, req keyResultRequest) (Objective, error) {
	if req.StartValue == req.TargetValue {
		return Objective{}, errTargetEqual
	}
	return s.change(ctx, p, objectiveID, "objective.key_result_updated", func(q *sqlc.Queries, o sqlc.Objective) error {
		kr, err := keyResult(ctx, q, o.ID, id)
		if err != nil {
			return err
		}
		if req.Version != kr.Version {
			return apperr.VersionConflict(kr.Version)
		}
		current := kr.CurrentValue
		switch {
		case req.CalcMode == ModeDerived && req.CurrentValue != nil && *req.CurrentValue != kr.CurrentValue:
			return errDerived // RF-111
		case req.CalcMode == ModeManual && req.CurrentValue != nil:
			current = *req.CurrentValue
		}
		_, err = q.UpdateKeyResult(ctx, sqlc.UpdateKeyResultParams{
			ID: id, Version: req.Version, Title: req.Title, Unit: req.Unit, StartValue: req.StartValue,
			TargetValue: req.TargetValue, CurrentValue: current, CalcMode: req.CalcMode, Position: req.Position,
		})
		return err
	})
}

func (s *Service) DeleteKeyResult(ctx context.Context, p *authz.Principal, objectiveID, id uuid.UUID) (Objective, error) {
	return s.change(ctx, p, objectiveID, "objective.key_result_deleted", func(q *sqlc.Queries, o sqlc.Objective) error {
		if _, err := keyResult(ctx, q, o.ID, id); err != nil {
			return err
		}
		if o.Status == StatusActive {
			n, err := q.CountKeyResults(ctx, o.ID)
			if err != nil {
				return fmt.Errorf("contar resultados clave: %w", err)
			}
			if n <= 1 {
				return errNoKeyResults
			}
		}
		return q.DeleteKeyResult(ctx, id)
	})
}

func keyResult(ctx context.Context, q *sqlc.Queries, objectiveID, id uuid.UUID) (sqlc.KeyResult, error) {
	kr, err := q.GetKeyResult(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && kr.ObjectiveID != objectiveID) {
		return kr, apperr.ErrNotFound
	}
	if err != nil {
		return kr, fmt.Errorf("leer resultado clave: %w", err)
	}
	return kr, nil
}

// change runs a change of a manageable objective, audits it and returns the
// new view.
func (s *Service) change(ctx context.Context, p *authz.Principal, id uuid.UUID, action string, fn func(*sqlc.Queries, sqlc.Objective) error) (Objective, error) {
	var out Objective
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		o, err := manageable(ctx, q, p, id)
		if err != nil {
			return err
		}
		if err := fn(q, o); err != nil {
			return err
		}
		if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: action, TargetType: "objective", TargetID: id.String()}); err != nil {
			return err
		}
		out, err = view(ctx, q, p, o)
		return err
	})
	return out, err
}

// Links (RF-104 to RF-109, RF-127) ------------------------------------------------

// linkableProject returns a visible project that is not a template.
func linkableProject(ctx context.Context, q *sqlc.Queries, p *authz.Principal, id uuid.UUID) (sqlc.Project, error) {
	pr, err := projects.Get(ctx, q, p, id, false)
	if errors.Is(err, apperr.ErrNotFound) {
		return pr, apperr.ErrValidation.WithDetail("field", "id")
	}
	if err != nil {
		return pr, err
	}
	if pr.IsTemplate {
		return pr, errTemplate
	}
	return pr, nil
}

func (s *Service) LinkProject(ctx context.Context, p *authz.Principal, id, projectID uuid.UUID, linked bool) (Objective, error) {
	return s.change(ctx, p, id, linkAction("project", linked), func(q *sqlc.Queries, o sqlc.Objective) error {
		if !linked {
			return unlinked(q.UnlinkObjectiveProject(ctx, sqlc.UnlinkObjectiveProjectParams{ObjectiveID: id, ProjectID: projectID}))
		}
		if _, err := linkableProject(ctx, q, p, projectID); err != nil {
			return err
		}
		return q.LinkObjectiveProject(ctx, sqlc.LinkObjectiveProjectParams{ObjectiveID: id, ProjectID: projectID})
	})
}

// LinkMilestone makes a milestone contribute to the objective and links its
// project too (RF-105, RF-106); unlinking drops it from the key results
// (RF-109, by cascade).
func (s *Service) LinkMilestone(ctx context.Context, p *authz.Principal, id, milestoneID uuid.UUID, linked bool) (Objective, error) {
	return s.change(ctx, p, id, linkAction("milestone", linked), func(q *sqlc.Queries, o sqlc.Objective) error {
		if !linked {
			return unlinked(q.UnlinkObjectiveMilestone(ctx, sqlc.UnlinkObjectiveMilestoneParams{ObjectiveID: id, MilestoneID: milestoneID}))
		}
		m, err := q.GetMilestone(ctx, milestoneID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrValidation.WithDetail("field", "id")
		}
		if err != nil {
			return fmt.Errorf("leer hito: %w", err)
		}
		if _, err := linkableProject(ctx, q, p, m.ProjectID); err != nil {
			return err
		}
		if err := q.LinkObjectiveMilestone(ctx, sqlc.LinkObjectiveMilestoneParams{ObjectiveID: id, MilestoneID: milestoneID}); err != nil {
			return err
		}
		return q.LinkObjectiveProject(ctx, sqlc.LinkObjectiveProjectParams{ObjectiveID: id, ProjectID: m.ProjectID})
	})
}

// LinkKeyResultWorkItem links a work item to a key result and its project to
// the objective (RF-106, RF-107).
func (s *Service) LinkKeyResultWorkItem(ctx context.Context, p *authz.Principal, id, krID, workItemID uuid.UUID, linked bool) (Objective, error) {
	return s.change(ctx, p, id, linkAction("key_result_work_item", linked), func(q *sqlc.Queries, o sqlc.Objective) error {
		if _, err := keyResult(ctx, q, id, krID); err != nil {
			return err
		}
		if !linked {
			return unlinked(q.UnlinkKeyResultWorkItem(ctx, sqlc.UnlinkKeyResultWorkItemParams{KeyResultID: krID, WorkItemID: workItemID}))
		}
		wi, _, err := workitems.Load(ctx, q, p, workItemID, false)
		if errors.Is(err, apperr.ErrNotFound) {
			return apperr.ErrValidation.WithDetail("field", "id")
		}
		if err != nil {
			return err
		}
		if _, err := linkableProject(ctx, q, p, wi.ProjectID); err != nil {
			return err
		}
		if err := q.LinkKeyResultWorkItem(ctx, sqlc.LinkKeyResultWorkItemParams{KeyResultID: krID, WorkItemID: workItemID}); err != nil {
			return err
		}
		return q.LinkObjectiveProject(ctx, sqlc.LinkObjectiveProjectParams{ObjectiveID: id, ProjectID: wi.ProjectID})
	})
}

// LinkKeyResultMilestone links a milestone already contributing to the
// objective to one of its key results (RF-107, RF-108).
func (s *Service) LinkKeyResultMilestone(ctx context.Context, p *authz.Principal, id, krID, milestoneID uuid.UUID, linked bool) (Objective, error) {
	return s.change(ctx, p, id, linkAction("key_result_milestone", linked), func(q *sqlc.Queries, o sqlc.Objective) error {
		if _, err := keyResult(ctx, q, id, krID); err != nil {
			return err
		}
		if !linked {
			return unlinked(q.UnlinkKeyResultMilestone(ctx, sqlc.UnlinkKeyResultMilestoneParams{KeyResultID: krID, MilestoneID: milestoneID}))
		}
		in, err := q.ObjectiveHasMilestone(ctx, sqlc.ObjectiveHasMilestoneParams{ObjectiveID: id, MilestoneID: milestoneID})
		if err != nil {
			return fmt.Errorf("comprobar hito: %w", err)
		}
		if !in {
			return errNotInObjective
		}
		return q.LinkKeyResultMilestone(ctx, sqlc.LinkKeyResultMilestoneParams{KeyResultID: krID, ObjectiveID: id, MilestoneID: milestoneID})
	})
}

func linkAction(what string, linked bool) string {
	if linked {
		return "objective." + what + "_linked"
	}
	return "objective." + what + "_unlinked"
}

func unlinked(n int64, err error) error {
	if err == nil && n == 0 {
		return apperr.ErrNotFound
	}
	return err
}

// Progress (RF-103, RF-110, RF-112, RF-113) ---------------------------------------------

// keyResultView computes the progress of a key result; derived ones get their
// current value from the share of linked elements done, all weighing the same.
func keyResultView(r sqlc.KeyResultsWithCountsRow) KeyResult {
	kr := KeyResult{
		ID: r.ID, Title: r.Title, Unit: r.Unit, StartValue: r.StartValue, TargetValue: r.TargetValue,
		CurrentValue: r.CurrentValue, CalcMode: r.CalcMode, Position: r.Position, Version: r.Version,
		LinkedTotal: r.WorkItemsTotal + r.MilestonesTotal, LinkedDone: r.WorkItemsDone + r.MilestonesDone, Links: []Link{},
	}
	span := r.TargetValue - r.StartValue
	if r.CalcMode == ModeDerived {
		fraction := 0.0
		if kr.LinkedTotal > 0 {
			fraction = float64(kr.LinkedDone) / float64(kr.LinkedTotal)
		}
		kr.CurrentValue = r.StartValue + span*fraction
	}
	kr.Progress = int(math.Round(math.Max(0, math.Min(1, (kr.CurrentValue-r.StartValue)/span)) * 100))
	return kr
}

func average(krs []KeyResult) int {
	if len(krs) == 0 {
		return 0
	}
	sum := 0
	for _, kr := range krs {
		sum += kr.Progress
	}
	return int(math.Round(float64(sum) / float64(len(krs))))
}

func view(ctx context.Context, q *sqlc.Queries, p *authz.Principal, o sqlc.Objective) (Objective, error) {
	out := Objective{
		ID: o.ID, Title: o.Title, Description: o.Description, Owner: workcore.Ref{ID: o.OwnerID}, Scope: o.Scope,
		PeriodStart: workcore.Date{Time: o.PeriodStart}, PeriodEnd: workcore.Date{Time: o.PeriodEnd},
		Status: o.Status, Version: o.Version, CreatedAt: o.CreatedAt,
		KeyResults: []KeyResult{}, Projects: []ProjectLink{}, Milestones: []MilestoneLink{},
	}
	r, err := ref(ctx, q, o)
	if err != nil {
		return out, err
	}
	out.Can.Manage = p.CanManageObjective(r) && o.Status != StatusClosed && o.Status != StatusCancelled
	people, err := q.WorkItemPeople(ctx, []uuid.UUID{o.OwnerID})
	if err != nil {
		return out, fmt.Errorf("leer responsable: %w", err)
	}
	if len(people) == 1 {
		out.Owner.Name = people[0].Name
	}
	if o.ScopeTeamID != nil {
		t, err := q.GetTeam(ctx, *o.ScopeTeamID)
		if err != nil {
			return out, fmt.Errorf("leer equipo: %w", err)
		}
		out.ScopeTeam = &workcore.Ref{ID: t.ID, Name: t.Name}
	}
	if o.ScopeProjectID != nil {
		pr, err := q.GetProject(ctx, *o.ScopeProjectID)
		if err != nil {
			return out, fmt.Errorf("leer proyecto: %w", err)
		}
		out.ScopeProject = &workcore.Ref{ID: pr.ID, Name: pr.Name}
	}

	rows, err := q.KeyResultsWithCounts(ctx, []uuid.UUID{o.ID})
	if err != nil {
		return out, fmt.Errorf("leer resultados clave: %w", err)
	}
	links, err := q.KeyResultLinks(ctx, o.ID)
	if err != nil {
		return out, fmt.Errorf("leer vínculos: %w", err)
	}
	for _, row := range rows {
		kr := keyResultView(row)
		for _, l := range links {
			if l.KeyResultID == kr.ID {
				kr.Links = append(kr.Links, Link{Kind: l.Kind, ID: l.ID, Label: l.Label, Name: l.Name})
			}
		}
		out.KeyResults = append(out.KeyResults, kr)
	}
	out.Progress = average(out.KeyResults)

	prs, err := q.ObjectiveProjects(ctx, o.ID)
	if err != nil {
		return out, fmt.Errorf("leer proyectos: %w", err)
	}
	for _, pr := range prs {
		out.Projects = append(out.Projects, ProjectLink{ID: pr.ID, Identifier: pr.Identifier, Name: pr.Name, Lifecycle: pr.Lifecycle})
	}
	ms, err := q.ObjectiveMilestones(ctx, o.ID)
	if err != nil {
		return out, fmt.Errorf("leer hitos: %w", err)
	}
	for _, m := range ms {
		out.Milestones = append(out.Milestones, MilestoneLink{
			ID: m.ID, Name: m.Name, Status: m.Status, DueDate: workcore.FromTime(m.DueDate), CompletedOn: workcore.FromTime(m.CompletedOn),
			Project: ProjectLink{ID: m.ProjectID, Identifier: m.Identifier, Name: m.ProjectName},
		})
	}
	return out, nil
}

func auditView(o sqlc.Objective) map[string]any {
	return map[string]any{
		"title": o.Title, "owner_id": o.OwnerID, "scope": o.Scope, "scope_team_id": o.ScopeTeamID,
		"scope_project_id": o.ScopeProjectID, "period_start": workcore.Date{Time: o.PeriodStart},
		"period_end": workcore.Date{Time: o.PeriodEnd}, "status": o.Status,
	}
}
