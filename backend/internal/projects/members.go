package projects

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/authz"
	"openscum/internal/db/sqlc"
	"openscum/internal/workcore"
	"openscum/internal/workload"
)

// Team of a project (spec 008, RF-33 to RF-54).

// Member is one person of the project's team, with their load.
type Member struct {
	User     workcore.Ref      `json:"user"`
	Status   string            `json:"status"`
	Role     authz.ProjectRole `json:"role"`
	Owner    bool              `json:"owner"`    // the owner is PM (RF-35)
	InTeam   bool              `json:"in_team"`  // a participant can't be removed (RF-38)
	Explicit bool              `json:"explicit"` // the role was given on purpose
	CanWork  bool              `json:"can_work"` // active and not only viewer (RF-37)
	Capacity *int32            `json:"capacity"` // weekly, in weight points (RF-46)
	Load     Load              `json:"load"`
}

// Load is the open weight assigned to someone (RF-48, RF-49).
type Load struct {
	Project    int64 `json:"project"`
	Total      int64 `json:"total"`
	Items      int32 `json:"items"`
	Overloaded bool  `json:"overloaded"`
}

type memberRequest struct {
	UserID uuid.UUID         `json:"user_id" validate:"required"`
	Role   authz.ProjectRole `json:"role" validate:"required,oneof=pm tech_lead member observer"`
}

type roleRequest struct {
	Role authz.ProjectRole `json:"role" validate:"required,oneof=pm tech_lead member observer"`
}

var (
	errInvalidMember = apperr.New(http.StatusBadRequest, "invalid_member", "Solo puede tener ese papel una persona activa que no sea solo observadora")
	errOwnerIsPM     = apperr.New(http.StatusConflict, "owner_is_pm", "El responsable del proyecto es su PM")
	errMemberInTeam  = apperr.New(http.StatusConflict, "member_from_team", "Pertenece al equipo del proyecto: ponle el papel observador")
)

// Members lists the project's team for those who manage it (RF-4, RF-33).
// Members lists the team of a project to whoever sees it; changing it is for
// whoever manages it (spec 023, RF-12).
func (s *Service) Members(ctx context.Context, p *authz.Principal, id uuid.UUID) ([]Member, error) {
	row, err := Get(ctx, s.store.Queries, p, id, false)
	if err != nil {
		return nil, err
	}
	return members(ctx, s.store.Queries, row)
}

// TeamSchedule is the schedule by person of the project's team, without its
// observers nor who can only read, for those who manage it (spec 021, RF-16 to
// RF-18). Somebody deactivated stays while a member.
func (s *Service) TeamSchedule(ctx context.Context, p *authz.Principal, id uuid.UUID, today time.Time) ([]workload.Row, error) {
	row, err := Get(ctx, s.store.Queries, p, id, false)
	if err != nil {
		return nil, err
	}
	if !p.CanManageProject(Ref(row)) { // RF-13
		return nil, apperr.ErrForbidden
	}
	ms, err := members(ctx, s.store.Queries, row)
	if err != nil {
		return nil, err
	}
	people := []workcore.Ref{}
	ids := []uuid.UUID{}
	for _, m := range ms {
		if m.Role != authz.ProjectRoleObserver && (m.CanWork || m.Status != "active") {
			people = append(people, m.User)
			ids = append(ids, m.User.ID)
		}
	}
	items, err := s.store.Queries.PersonScheduleItems(ctx, sqlc.PersonScheduleItemsParams{Ids: ids, ProjectID: &id})
	if err != nil {
		return nil, fmt.Errorf("cronograma por persona: %w", err)
	}
	return workload.Build(people, items, today), nil
}

// members builds the team: owner, participants and people added on purpose.
func members(ctx context.Context, q *sqlc.Queries, row sqlc.Project) ([]Member, error) {
	team, err := q.ProjectTeamParticipants(ctx, row.TeamID)
	if err != nil {
		return nil, fmt.Errorf("participantes: %w", err)
	}
	explicit, err := q.ListExplicitProjectMembers(ctx, row.ID)
	if err != nil {
		return nil, fmt.Errorf("miembros: %w", err)
	}
	roles := map[uuid.UUID]authz.ProjectRole{}
	for _, m := range explicit {
		roles[m.UserID] = authz.ProjectRole(m.Role)
	}
	ids := []uuid.UUID{row.OwnerID}
	for _, u := range team {
		if !slices.Contains(ids, u) {
			ids = append(ids, u)
		}
	}
	for u := range roles {
		if !slices.Contains(ids, u) {
			ids = append(ids, u)
		}
	}
	people, err := q.TeamPeople(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("personas: %w", err)
	}
	loads, err := q.OpenLoad(ctx, sqlc.OpenLoadParams{ProjectID: row.ID, Ids: ids})
	if err != nil {
		return nil, fmt.Errorf("carga: %w", err)
	}
	byUser := map[uuid.UUID]sqlc.OpenLoadRow{}
	for _, l := range loads {
		byUser[l.UserID] = l
	}
	out := make([]Member, 0, len(people))
	for _, u := range people {
		m := Member{
			User: workcore.Ref{ID: u.ID, Name: u.Name}, Status: u.Status, Owner: u.ID == row.OwnerID,
			InTeam: slices.Contains(team, u.ID), Capacity: u.WeeklyCapacity,
			CanWork: u.Status == "active" && slices.ContainsFunc(u.Roles, func(r string) bool { return authz.Role(r) != authz.RoleViewer }),
		}
		r, explicit := roles[u.ID]
		m.Explicit = explicit
		switch {
		case m.Owner:
			m.Role = authz.ProjectRolePM
		case explicit:
			m.Role = r
		default:
			m.Role = authz.ProjectRoleMember
		}
		l := byUser[u.ID]
		m.Load = Load{Project: l.ProjectWeight, Total: l.TotalWeight, Items: l.ProjectItems}
		m.Load.Overloaded = m.Capacity != nil && l.TotalWeight > int64(*m.Capacity)
		out = append(out, m)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Load.Total != out[j].Load.Total {
			return out[i].Load.Total > out[j].Load.Total
		}
		return out[i].User.Name < out[j].User.Name
	})
	return out, nil
}

// SetMember adds someone with a role or changes their role (RF-33 to RF-37, RF-45).
func (s *Service) SetMember(ctx context.Context, p *authz.Principal, id, userID uuid.UUID, role authz.ProjectRole, adding bool) ([]Member, error) {
	var out []Member
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		row, err := s.teamChange(ctx, q, p, id)
		if err != nil {
			return err
		}
		if userID == row.OwnerID {
			return errOwnerIsPM
		}
		if err := checkMember(ctx, q, userID, role); err != nil {
			return err
		}
		before, err := q.GetProjectMember(ctx, sqlc.GetProjectMemberParams{ProjectID: id, UserID: userID})
		had := err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("leer miembro: %w", err)
		}
		if !adding && !had {
			team, err := q.ProjectTeamParticipants(ctx, row.TeamID)
			if err != nil {
				return fmt.Errorf("participantes: %w", err)
			}
			if !slices.Contains(team, userID) {
				return apperr.ErrNotFound
			}
		}
		if err := q.UpsertProjectMember(ctx, sqlc.UpsertProjectMemberParams{ProjectID: id, UserID: userID, Role: string(role)}); err != nil {
			return fmt.Errorf("guardar miembro: %w", err)
		}
		entry := audit.Entry{Actor: audit.ByUser(p), Action: "project.member_added", TargetType: "project", TargetID: id.String(),
			After: map[string]any{"user_id": userID, "role": role}}
		if had {
			entry.Action, entry.Before = "project.member_role_changed", map[string]any{"user_id": userID, "role": before.Role}
		}
		if err := audit.Record(ctx, q, entry); err != nil {
			return err
		}
		out, err = members(ctx, q, row)
		return err
	})
	if err == nil {
		s.invalidate(userID)
	}
	return out, err
}

// RemoveMember takes out someone added on purpose; a participant stays (RF-38).
func (s *Service) RemoveMember(ctx context.Context, p *authz.Principal, id, userID uuid.UUID) ([]Member, error) {
	var out []Member
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		row, err := s.teamChange(ctx, q, p, id)
		if err != nil {
			return err
		}
		if userID == row.OwnerID {
			return errOwnerIsPM
		}
		team, err := q.ProjectTeamParticipants(ctx, row.TeamID)
		if err != nil {
			return fmt.Errorf("participantes: %w", err)
		}
		if slices.Contains(team, userID) {
			return errMemberInTeam
		}
		before, err := q.GetProjectMember(ctx, sqlc.GetProjectMemberParams{ProjectID: id, UserID: userID})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("leer miembro: %w", err)
		}
		if _, err := q.DeleteProjectMember(ctx, sqlc.DeleteProjectMemberParams{ProjectID: id, UserID: userID}); err != nil {
			return fmt.Errorf("quitar miembro: %w", err)
		}
		if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "project.member_removed", TargetType: "project",
			TargetID: id.String(), Before: map[string]any{"user_id": userID, "role": before.Role}}); err != nil {
			return err
		}
		out, err = members(ctx, q, row)
		return err
	})
	if err == nil {
		s.invalidate(userID)
	}
	return out, err
}

// teamChange loads a writable project whose team the caller manages.
func (s *Service) teamChange(ctx context.Context, q *sqlc.Queries, p *authz.Principal, id uuid.UUID) (sqlc.Project, error) {
	row, err := Get(ctx, q, p, id, true)
	if err != nil {
		return row, err
	}
	if !p.CanManageProject(Ref(row)) {
		return row, apperr.ErrForbidden
	}
	return row, Writable(row)
}

// checkMember: a working role needs someone active and not only viewer; an
// observer only needs someone who hasn't left (RF-37).
func checkMember(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, role authz.ProjectRole) error {
	if role != authz.ProjectRoleObserver {
		return workcore.CheckWorker(ctx, q, userID, errInvalidMember)
	}
	u, err := q.GetUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && u.Status == "offboarded") {
		return errInvalidMember
	}
	if err != nil {
		return fmt.Errorf("leer usuario: %w", err)
	}
	return nil
}

func (s *Service) invalidate(userID uuid.UUID) {
	if s.onRolesChanged != nil {
		s.onRolesChanged(userID)
	}
}

// Suggestion is an open work item without assignees and who could take it (RF-51, RF-52).
type Suggestion struct {
	ID       uuid.UUID      `json:"id"`
	Key      string         `json:"key"`
	Title    string         `json:"title"`
	Weight   int16          `json:"weight"`
	DueDate  *workcore.Date `json:"due_date"`
	LabelIDs []uuid.UUID    `json:"label_ids"`
	People   []workcore.Ref `json:"people"`
}

// Suggestions proposes up to 2 working members per unassigned work item: the
// ones sharing a label first, then by load against capacity (RF-52).
func (s *Service) Suggestions(ctx context.Context, p *authz.Principal, id uuid.UUID) ([]Suggestion, error) {
	q := s.store.Queries
	row, err := Get(ctx, q, p, id, false)
	if err != nil {
		return nil, err
	}
	if !p.CanManageProject(Ref(row)) {
		return nil, apperr.ErrForbidden
	}
	team, err := members(ctx, q, row)
	if err != nil {
		return nil, err
	}
	var workers []Member
	ids := []uuid.UUID{}
	for _, m := range team {
		if m.CanWork && m.Role != authz.ProjectRoleObserver {
			workers = append(workers, m)
			ids = append(ids, m.User.ID)
		}
	}
	pairs, err := q.PeopleLabels(ctx, sqlc.PeopleLabelsParams{ProjectID: id, Ids: ids})
	if err != nil {
		return nil, fmt.Errorf("etiquetas de las personas: %w", err)
	}
	labelsOf := map[uuid.UUID][]uuid.UUID{}
	for _, pl := range pairs {
		labelsOf[pl.UserID] = append(labelsOf[pl.UserID], pl.LabelID)
	}
	// Load against capacity; without capacity, after everyone who has one.
	ratio := func(m Member) float64 {
		if m.Capacity == nil || *m.Capacity == 0 {
			return 1e9 + float64(m.Load.Total)
		}
		return float64(m.Load.Total) / float64(*m.Capacity)
	}
	items, err := q.UnassignedOpenItems(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("tareas sin asignar: %w", err)
	}
	out := make([]Suggestion, 0, len(items))
	for _, it := range items {
		shares := func(m Member) bool {
			return slices.ContainsFunc(it.LabelIds, func(l uuid.UUID) bool { return slices.Contains(labelsOf[m.User.ID], l) })
		}
		ranked := slices.Clone(workers)
		sort.SliceStable(ranked, func(i, j int) bool {
			if a, b := shares(ranked[i]), shares(ranked[j]); a != b {
				return a
			}
			return ratio(ranked[i]) < ratio(ranked[j])
		})
		sg := Suggestion{ID: it.ID, Key: fmt.Sprintf("%s-%d", row.Identifier, it.SequenceNumber), Title: it.Title, Weight: it.Weight,
			DueDate: workcore.FromTime(it.DueDate), LabelIDs: it.LabelIds, People: []workcore.Ref{}}
		for _, m := range ranked[:min(2, len(ranked))] {
			sg.People = append(sg.People, m.User)
		}
		out = append(out, sg)
	}
	return out, nil
}
