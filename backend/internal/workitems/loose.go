package workitems

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/authz"
	"openscum/internal/db/sqlc"
	"openscum/internal/files"
	"openscum/internal/loose"
	"openscum/internal/projects"
	"openscum/internal/workcore"
)

// Work items without a project (spec 022).

var (
	errTaken      = apperr.New(http.StatusConflict, "already_taken", "Ya la tomó otra persona")
	errNotTaken   = apperr.New(http.StatusConflict, "not_taken", "No es una tarea tomada por ti")
	errEmptyTeam  = apperr.New(http.StatusConflict, "team_without_members", "El equipo no tiene a nadie")
	errNotMovable = apperr.New(http.StatusConflict, "not_movable", "Esta tarea no se puede pasar a un proyecto")
)

// Loose is what a work item without a project adds to its view.
type Loose struct {
	Team *workcore.Ref `json:"team"`
	Mode string        `json:"mode"` // "", claim or each
	// The «each person» group: its progress, from the group or a child (RF-24).
	Group *Ref `json:"group"`
	Done  int  `json:"done"`
	Total int  `json:"total"`
	Can   struct {
		Claim   bool `json:"claim"`
		Release bool `json:"release"`
		Move    bool `json:"move"`
	} `json:"can"`
}

func looseView(ctx context.Context, q *sqlc.Queries, p *authz.Principal, row sqlc.WorkItem, a loose.Access) (*Loose, error) {
	out := &Loose{}
	open := row.ClosedAt == nil
	if row.TeamID != nil {
		team, err := q.GetTeam(ctx, *row.TeamID)
		if err != nil {
			return nil, fmt.Errorf("leer equipo: %w", err)
		}
		out.Team, out.Mode = &workcore.Ref{ID: team.ID, Name: team.Name}, *row.TeamMode
	}
	assignees, err := assigneeIDs(ctx, q, row.ID)
	if err != nil {
		return nil, err
	}
	switch out.Mode {
	case loose.ModeEach:
		group := row.ID
		if row.ParentID != nil {
			group = *row.ParentID
		}
		gs, err := refs(ctx, q, []uuid.UUID{group})
		if err != nil {
			return nil, err
		}
		g := gs[group]
		out.Group = &g
		progress, err := q.GroupProgress(ctx, []uuid.UUID{group})
		if err != nil {
			return nil, fmt.Errorf("avance del grupo: %w", err)
		}
		if len(progress) == 1 {
			out.Done, out.Total = int(progress[0].Done), int(progress[0].Total)
		}
	case loose.ModeClaim:
		out.Can.Claim = open && len(assignees) == 0 && p.Has(authz.LooseCreateOwn) && p.ParticipantTeams[*row.TeamID]
		out.Can.Release = open && slices.Equal(assignees, []uuid.UUID{p.UserID})
	}
	out.Can.Move = open && a.Write && out.Mode != loose.ModeEach
	return out, nil
}

type looseRequest struct {
	Title       string          `json:"title" validate:"required,max=500"`
	Description json.RawMessage `json:"description"`
	DueDate     *workcore.Date  `json:"due_date"`
	Weight      *int16          `json:"weight"`
	PriorityID  *uuid.UUID      `json:"priority_id"`
	// One of the two (RF-3).
	UserID *uuid.UUID `json:"user_id"`
	TeamID *uuid.UUID `json:"team_id"`
	Mode   string     `json:"mode" validate:"omitempty,oneof=claim each"`
}

// CreateLoose adds a task without a project for a person or a team
// (RF-1 to RF-11). An «each person» one returns the group.
func (s *Service) CreateLoose(ctx context.Context, p *authz.Principal, req looseRequest) (WorkItem, error) {
	if (req.UserID == nil) == (req.TeamID == nil) {
		return WorkItem{}, apperr.ErrValidation.WithDetail("field", "user_id")
	}
	if req.TeamID != nil && req.Mode == "" {
		return WorkItem{}, apperr.ErrValidation.WithDetail("field", "mode")
	}
	if err := checkWeight(req.Weight); err != nil {
		return WorkItem{}, err
	}
	var out WorkItem
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		var team sqlc.Team
		if req.UserID != nil {
			ok, err := loose.CanAssign(ctx, q, p, *req.UserID)
			if err != nil {
				return err
			}
			if !ok {
				return apperr.ErrNotFound
			}
			if err := workcore.CheckWorker(ctx, q, *req.UserID, errInvalidAssignee); err != nil {
				return err
			}
		} else {
			t, err := q.GetTeam(ctx, *req.TeamID)
			if errors.Is(err, pgx.ErrNoRows) || (err == nil && !loose.CanTeam(p, t)) {
				return apperr.ErrNotFound
			}
			if err != nil {
				return fmt.Errorf("leer equipo: %w", err)
			}
			team = t
		}
		var members []sqlc.TeamMembersForTasksRow
		if req.Mode == loose.ModeEach {
			var err error
			if members, err = q.TeamMembersForTasks(ctx, team.ID); err != nil {
				return fmt.Errorf("miembros del equipo: %w", err)
			}
			if len(members) == 0 {
				return errEmptyTeam
			}
		}
		inbox, err := loose.Inbox(ctx, q, p.UserID)
		if err != nil {
			return err
		}
		typ, err := q.LooseTaskType(ctx)
		if err != nil {
			return fmt.Errorf("tipo de tarea: %w", err)
		}
		weight := typ.DefaultWeight
		if req.Weight != nil {
			weight = *req.Weight
		}
		priority := req.PriorityID
		if priority == nil {
			def, err := q.GetDefaultPriority(ctx)
			if err != nil {
				return fmt.Errorf("prioridad por defecto: %w", err)
			}
			priority = &def.ID
		} else if err := checkPriority(ctx, q, *priority); err != nil {
			return err
		}
		status, err := initialStatus(ctx, q, nil)
		if err != nil {
			return err
		}
		desc, text, err := files.NewPage(req.Description)
		if err != nil {
			return err
		}
		f := sqlc.CreateWorkItemParams{
			TypeID: typ.ID, StatusID: status, PriorityID: *priority, Title: req.Title, Description: desc,
			DescriptionText: text, AuthorID: p.UserID, DueDate: workcore.ToTime(req.DueDate), Weight: weight,
		}
		var teamID *uuid.UUID
		if req.TeamID != nil {
			teamID = &team.ID
		}
		row, err := loose.Insert(ctx, q, audit.ByUser(p), inbox, f, req.UserID, teamID, req.Mode)
		if err != nil {
			return err
		}
		for _, m := range members {
			if _, err := loose.Child(ctx, q, audit.ByUser(p), inbox, row, m.ID); err != nil {
				return err
			}
		}
		if row, err = q.GetWorkItem(ctx, row.ID); err != nil {
			return fmt.Errorf("releer tarea: %w", err)
		}
		out, err = view(ctx, q, p, row, inbox)
		return err
	})
	return out, err
}

// Claim gives a «one person takes it» task to the caller, member of its team;
// of two at once, only the first gets it (RF-19, RF-20).
func (s *Service) Claim(ctx context.Context, p *authz.Principal, id uuid.UUID) (WorkItem, error) {
	return s.take(ctx, p, id, true)
}

// Release gives back to the team a task the caller took (RF-22).
func (s *Service) Release(ctx context.Context, p *authz.Principal, id uuid.UUID) (WorkItem, error) {
	return s.take(ctx, p, id, false)
}

func (s *Service) take(ctx context.Context, p *authz.Principal, id uuid.UUID, claim bool) (WorkItem, error) {
	var out WorkItem
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		row, project, err := Load(ctx, q, p, id, true)
		if err != nil {
			return err
		}
		a, err := Access(ctx, q, p, row, project)
		if err != nil {
			return err
		}
		lv, err := looseView(ctx, q, p, row, a)
		if err != nil {
			return err
		}
		current, err := assigneeIDs(ctx, q, id)
		if err != nil {
			return err
		}
		next := []uuid.UUID{}
		switch {
		case claim && lv.Can.Claim:
			next = []uuid.UUID{p.UserID}
		case claim && project.IsInbox && lv.Mode == loose.ModeClaim && len(current) > 0:
			return errTaken
		case claim:
			return apperr.ErrForbidden
		case !lv.Can.Release:
			return errNotTaken
		}
		if err := setAssignees(ctx, q, id, next); err != nil {
			return err
		}
		updated, err := q.SetWorkItemAssigneeOnly(ctx, sqlc.SetWorkItemAssigneeOnlyParams{ID: id, AssigneeID: first(next)})
		if err != nil {
			return fmt.Errorf("asignar tarea: %w", err)
		}
		key := Key(project.Identifier, row.SequenceNumber)
		action := "work_item.claimed"
		if !claim {
			action = "work_item.released"
		}
		if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: action, TargetType: "work_item", TargetID: id.String(),
			Before: map[string]any{"key": key, "assignee_ids": current}, After: map[string]any{"key": key, "assignee_ids": next}}); err != nil {
			return err
		}
		out, err = view(ctx, q, p, updated, project)
		return err
	})
	return out, err
}

// Claimable are the open tasks nobody took of the caller's teams (RF-18).
type Claimable struct {
	ID      uuid.UUID      `json:"id"`
	Key     string         `json:"key"`
	Title   string         `json:"title"`
	DueDate *workcore.Date `json:"due_date"`
	Weight  int16          `json:"weight"`
	Team    workcore.Ref   `json:"team"`
	Author  workcore.Ref   `json:"author"`
}

func (s *Service) Claimable(ctx context.Context, p *authz.Principal) ([]Claimable, error) {
	out := []Claimable{}
	if !p.Has(authz.LooseCreateOwn) || len(p.ParticipantTeams) == 0 {
		return out, nil
	}
	inbox, err := s.store.GetInbox(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return nil, fmt.Errorf("leer bandeja: %w", err)
	}
	teams := make([]uuid.UUID, 0, len(p.ParticipantTeams))
	for t := range p.ParticipantTeams {
		teams = append(teams, t)
	}
	rows, err := s.store.ClaimableTasks(ctx, teams)
	if err != nil {
		return nil, fmt.Errorf("tareas para tomar: %w", err)
	}
	for _, r := range rows {
		out = append(out, Claimable{
			ID: r.ID, Key: Key(inbox.Identifier, r.SequenceNumber), Title: r.Title, DueDate: workcore.FromTime(r.DueDate),
			Weight: r.Weight, Team: workcore.Ref{ID: *r.TeamID, Name: r.TeamName}, Author: workcore.Ref{ID: r.AuthorID, Name: r.AuthorName},
		})
	}
	return out, nil
}

// TeamTask is a team task in Mi equipo (RF-33 to RF-35).
type TeamTask struct {
	ID        uuid.UUID      `json:"id"`
	Key       string         `json:"key"`
	Title     string         `json:"title"`
	DueDate   *workcore.Date `json:"due_date"`
	Mode      string         `json:"mode"`
	Team      workcore.Ref   `json:"team"`
	Closed    bool           `json:"closed"`
	CreatedAt time.Time      `json:"created_at"`
	// Who has it («one person takes it») or each member and whether they did
	// theirs («each person»).
	People []TeamTaskPerson `json:"people"`
	Done   int              `json:"done"`
	Total  int              `json:"total"`
}

type TeamTaskPerson struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Done bool      `json:"done"`
}

// TeamTasks are the team tasks of the teams the caller leads, or of all for
// whom may assign to anybody.
func (s *Service) TeamTasks(ctx context.Context, p *authz.Principal) ([]TeamTask, error) {
	out := []TeamTask{}
	if !p.Has(authz.LooseAssignAll) && !p.Has(authz.LooseAssignTeam) {
		return nil, apperr.ErrForbidden
	}
	inbox, err := s.store.GetInbox(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return nil, fmt.Errorf("leer bandeja: %w", err)
	}
	all, err := s.store.ListTeams(ctx)
	if err != nil {
		return nil, fmt.Errorf("equipos: %w", err)
	}
	teams := []uuid.UUID{}
	for _, t := range all {
		if p.Has(authz.LooseAssignAll) || p.ScopeTeams[t.ID] {
			teams = append(teams, t.ID)
		}
	}
	rows, err := s.store.TeamTasks(ctx, teams)
	if err != nil {
		return nil, fmt.Errorf("tareas del equipo: %w", err)
	}
	ids := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	people, err := s.store.TaskPeople(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("personas de las tareas: %w", err)
	}
	byTask := map[uuid.UUID][]TeamTaskPerson{}
	for _, r := range people {
		byTask[r.TaskID] = append(byTask[r.TaskID], TeamTaskPerson{ID: r.UserID, Name: r.Name, Done: r.IsClosed})
	}
	for _, r := range rows {
		t := TeamTask{
			ID: r.ID, Key: Key(inbox.Identifier, r.SequenceNumber), Title: r.Title, DueDate: workcore.FromTime(r.DueDate),
			Mode: *r.TeamMode, Team: workcore.Ref{ID: *r.TeamID, Name: r.TeamName}, Closed: r.IsClosed, CreatedAt: r.CreatedAt,
			People: byTask[r.ID],
		}
		if t.People == nil {
			t.People = []TeamTaskPerson{}
		}
		if t.Mode == loose.ModeEach {
			t.Total = len(t.People)
			for _, x := range t.People {
				if x.Done {
					t.Done++
				}
			}
		}
		out = append(out, t)
	}
	return out, nil
}

type moveRequest struct {
	ProjectID uuid.UUID `json:"project_id" validate:"required"`
	Version   int32     `json:"version" validate:"required"`
}

// Move takes a task without a project to an active project where the caller
// can create tasks, with the project's next key (RF-37 to RF-39).
func (s *Service) Move(ctx context.Context, p *authz.Principal, id uuid.UUID, req moveRequest) (WorkItem, error) {
	var out WorkItem
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		row, inbox, err := Load(ctx, q, p, id, true)
		if err != nil {
			return err
		}
		if !inbox.IsInbox {
			return errNotMovable
		}
		a, err := Access(ctx, q, p, row, inbox)
		if err != nil {
			return err
		}
		if !a.Write {
			return apperr.ErrForbidden
		}
		if row.TeamMode != nil && *row.TeamMode == loose.ModeEach {
			return errNotMovable
		}
		if req.Version != row.Version {
			return apperr.VersionConflict(row.Version)
		}
		project, err := projects.Get(ctx, q, p, req.ProjectID, true)
		if err != nil {
			return err
		}
		if project.IsTemplate || !p.CanWriteWorkItems(projects.Ref(project)) {
			return apperr.ErrForbidden
		}
		if err := projects.Writable(project); err != nil {
			return err
		}
		if _, err := checkType(ctx, q, project.ID, row.TypeID); err != nil {
			return err
		}
		n, err := q.NextWorkItemNumber(ctx, project.ID)
		if err != nil {
			return fmt.Errorf("numerar work item: %w", err)
		}
		moved, err := q.MoveWorkItemToProject(ctx, sqlc.MoveWorkItemToProjectParams{ID: id, ProjectID: project.ID, SequenceNumber: n})
		if err != nil {
			return fmt.Errorf("mover tarea: %w", err)
		}
		if err := q.MoveWorkItemFiles(ctx, sqlc.MoveWorkItemFilesParams{OwnerID: id, ProjectID: project.ID}); err != nil {
			return fmt.Errorf("mover archivos: %w", err)
		}
		if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "work_item.moved", TargetType: "work_item", TargetID: id.String(),
			Before: map[string]any{"key": Key(inbox.Identifier, row.SequenceNumber), "project_id": inbox.ID, "team_id": row.TeamID},
			After:  map[string]any{"key": Key(project.Identifier, n), "project_id": project.ID}}); err != nil {
			return err
		}
		out, err = view(ctx, q, p, moved, project)
		return err
	})
	return out, err
}
