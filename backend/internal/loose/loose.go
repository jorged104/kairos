// Package loose holds the work items without a project (spec 022). They live
// in a hidden project, the inbox, so they keep the statuses, comments, pages,
// history and points of any work item; who reaches them comes from the work
// item, not from that project.
package loose

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/authz"
	"openscum/internal/db/sqlc"
)

const (
	ModeClaim = "claim" // one person takes it (RF-18 to RF-23)
	ModeEach  = "each"  // each member does their own (RF-24 to RF-28)

	inboxIdentifier = "TAREA"
	inboxName       = "Tareas sin proyecto"
)

var errNoTeams = apperr.New(http.StatusConflict, "loose_needs_team", "Crea antes un equipo")

// Access is what the caller may do with a work item of the inbox.
type Access struct{ Read, Write, Delete bool }

// Of decides the access to a work item of the inbox from its creator, its
// assignees and its team (RF-14 to RF-17).
func Of(p *authz.Principal, row sqlc.WorkItem, assignees []uuid.UUID) Access {
	if p.Has(authz.LooseAssignAll) || row.AuthorID == p.UserID {
		return Access{Read: true, Write: true, Delete: true}
	}
	var out Access
	if slices.Contains(assignees, p.UserID) {
		out.Read, out.Write = true, true
	}
	if row.TeamID != nil {
		if p.Has(authz.LooseAssignTeam) && p.ScopeTeams[*row.TeamID] {
			out.Read, out.Write = true, true
		}
		// Members of the team or of a subteam: the team is among the
		// ancestors of theirs.
		if p.ParticipantTeams[*row.TeamID] {
			out.Read = true
		}
	}
	return out
}

// Inbox returns the hidden project, created by the first task without a
// project: owned by its creator, with a team of theirs if any.
func Inbox(ctx context.Context, q *sqlc.Queries, creator uuid.UUID) (sqlc.Project, error) {
	row, err := q.GetInbox(ctx)
	if err == nil || !errors.Is(err, pgx.ErrNoRows) {
		return row, err
	}
	if err := q.LockInboxCreation(ctx); err != nil {
		return row, fmt.Errorf("bloquear bandeja: %w", err)
	}
	if row, err = q.GetInbox(ctx); err == nil || !errors.Is(err, pgx.ErrNoRows) {
		return row, err
	}
	team, err := q.InboxTeam(ctx, creator)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, errNoTeams
	}
	if err != nil {
		return row, fmt.Errorf("equipo de la bandeja: %w", err)
	}
	identifier := inboxIdentifier
	for i := 2; ; i++ {
		taken, err := q.ProjectIdentifierTaken(ctx, identifier)
		if err != nil {
			return row, fmt.Errorf("identificador de la bandeja: %w", err)
		}
		if !taken {
			break
		}
		identifier = inboxIdentifier + strconv.Itoa(i)
	}
	if row, err = q.CreateInbox(ctx, sqlc.CreateInboxParams{TeamID: team, OwnerID: creator, Identifier: identifier, Name: inboxName}); err != nil {
		return row, fmt.Errorf("crear bandeja: %w", err)
	}
	types, err := q.ListWorkItemTypes(ctx)
	if err != nil {
		return row, fmt.Errorf("tipos: %w", err)
	}
	for _, t := range types {
		if err := q.AddProjectType(ctx, sqlc.AddProjectTypeParams{ProjectID: row.ID, TypeID: t.ID}); err != nil {
			return row, fmt.Errorf("tipo de la bandeja: %w", err)
		}
	}
	return row, nil
}

// Insert adds a work item to the inbox, assigned to whom it says, and records
// it in the audit.
func Insert(ctx context.Context, q *sqlc.Queries, actor audit.Actor, inbox sqlc.Project, f sqlc.CreateWorkItemParams,
	assignee *uuid.UUID, team *uuid.UUID, mode string) (sqlc.WorkItem, error) {
	n, err := q.NextWorkItemNumber(ctx, inbox.ID)
	if err != nil {
		return sqlc.WorkItem{}, fmt.Errorf("numerar tarea: %w", err)
	}
	f.ProjectID, f.SequenceNumber, f.AssigneeID = inbox.ID, n, assignee
	row, err := q.CreateWorkItem(ctx, f)
	if err != nil {
		return row, fmt.Errorf("crear tarea: %w", err)
	}
	if assignee != nil {
		if err := q.AddWorkItemAssignees(ctx, sqlc.AddWorkItemAssigneesParams{WorkItemID: row.ID, UserIds: []uuid.UUID{*assignee}}); err != nil {
			return row, fmt.Errorf("asignar tarea: %w", err)
		}
	}
	if team != nil {
		m := mode
		if err := q.SetWorkItemTeam(ctx, sqlc.SetWorkItemTeamParams{ID: row.ID, TeamID: team, TeamMode: &m}); err != nil {
			return row, fmt.Errorf("equipo de la tarea: %w", err)
		}
		row.TeamID, row.TeamMode = team, &m
	}
	if f.ParentID != nil {
		if err := q.RefreshWorkItemUp(ctx, row.ID); err != nil {
			return row, fmt.Errorf("recalcular derivados: %w", err)
		}
	}
	after := map[string]any{
		"key": inbox.Identifier + "-" + strconv.Itoa(int(n)), "title": row.Title, "status_id": row.StatusID,
		"priority_id": row.PriorityID, "due_date": row.DueDate, "weight": row.Weight, "parent_id": row.ParentID,
		"assignee_ids": []uuid.UUID{}, "team_id": team, "team_mode": row.TeamMode,
	}
	if assignee != nil {
		after["assignee_ids"] = []uuid.UUID{*assignee}
	}
	return row, audit.Record(ctx, q, audit.Entry{Actor: actor, Action: "work_item.created", TargetType: "work_item",
		TargetID: row.ID.String(), After: after})
}

// Child is the task of one member in an «each person» group (RF-9, RF-25).
func Child(ctx context.Context, q *sqlc.Queries, actor audit.Actor, inbox sqlc.Project, group sqlc.WorkItem, user uuid.UUID) (sqlc.WorkItem, error) {
	status, err := q.GetDefaultStatus(ctx)
	if err != nil {
		return sqlc.WorkItem{}, fmt.Errorf("estado por defecto: %w", err)
	}
	parent := group.ID
	return Insert(ctx, q, actor, inbox, sqlc.CreateWorkItemParams{
		TypeID: group.TypeID, StatusID: status.ID, PriorityID: group.PriorityID, ParentID: &parent,
		Title: group.Title, Description: group.Description, DescriptionText: group.DescriptionText, AuthorID: group.AuthorID,
		StartDate: group.StartDate, DueDate: group.DueDate, Weight: group.Weight, Depth: 1,
	}, &user, group.TeamID, ModeEach)
}

// Join gives someone who enters a team their task of each open «each person»
// group of that team or of its ancestors (RF-25). It runs in the transaction
// that adds them.
func Join(ctx context.Context, q *sqlc.Queries, actor audit.Actor, team, user uuid.UUID) error {
	inbox, err := q.GetInbox(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("leer bandeja: %w", err)
	}
	ok, err := canWork(ctx, q, user)
	if err != nil || !ok {
		return err
	}
	groups, err := q.OpenEachGroupsForMember(ctx, sqlc.OpenEachGroupsForMemberParams{UserID: user, TeamID: team})
	if err != nil {
		return fmt.Errorf("tareas del equipo: %w", err)
	}
	for _, g := range groups {
		if _, err := Child(ctx, q, actor, inbox, g, user); err != nil {
			return err
		}
	}
	return nil
}

// canWork: active and not only a viewer, as the members of TeamMembersForTasks.
func canWork(ctx context.Context, q *sqlc.Queries, user uuid.UUID) (bool, error) {
	u, err := q.GetUser(ctx, user)
	if err != nil {
		return false, fmt.Errorf("leer usuario: %w", err)
	}
	if u.Status != "active" {
		return false, nil
	}
	roles, err := q.ListUserRoles(ctx, user)
	if err != nil {
		return false, fmt.Errorf("leer roles: %w", err)
	}
	return slices.ContainsFunc(roles, func(r sqlc.ListUserRolesRow) bool { return authz.Role(r.Role) != authz.RoleViewer }), nil
}

// SyncGroup closes an «each person» group when all its children are closed
// and reopens it when one is reopened (RF-27).
func SyncGroup(ctx context.Context, q *sqlc.Queries, group uuid.UUID) error {
	progress, err := q.GroupProgress(ctx, []uuid.UUID{group})
	if err != nil {
		return fmt.Errorf("avance del grupo: %w", err)
	}
	g, err := q.GetWorkItemForUpdate(ctx, group)
	if err != nil {
		return fmt.Errorf("leer grupo: %w", err)
	}
	all := len(progress) == 1 && progress[0].Done == progress[0].Total
	closed := g.ClosedAt != nil
	switch {
	case all && !closed:
		st, err := q.FirstClosedStatus(ctx)
		if err != nil {
			return fmt.Errorf("estado cerrado: %w", err)
		}
		now := time.Now().UTC()
		return q.SetWorkItemStatusDirect(ctx, sqlc.SetWorkItemStatusDirectParams{ID: group, StatusID: st.ID, ClosedAt: &now})
	case !all && closed:
		st, err := q.GetDefaultStatus(ctx)
		if err != nil {
			return fmt.Errorf("estado por defecto: %w", err)
		}
		return q.SetWorkItemStatusDirect(ctx, sqlc.SetWorkItemStatusDirectParams{ID: group, StatusID: st.ID})
	}
	return nil
}

// CanAssign: oneself, anybody for whom may assign to all, the people of
// one's teams for a lead (RF-4, RF-23).
func CanAssign(ctx context.Context, q *sqlc.Queries, p *authz.Principal, user uuid.UUID) (bool, error) {
	if user == p.UserID || p.Has(authz.LooseAssignAll) {
		return true, nil
	}
	if !p.Has(authz.LooseAssignTeam) {
		return false, nil
	}
	roles, err := q.ListUserRoles(ctx, user)
	if err != nil {
		return false, fmt.Errorf("leer roles: %w", err)
	}
	teams, err := q.ListUserTeams(ctx, user)
	if err != nil {
		return false, fmt.Errorf("leer equipos: %w", err)
	}
	t := authz.Target{UserID: user}
	for _, r := range roles {
		t.Roles = append(t.Roles, authz.Role(r.Role))
	}
	for _, tm := range teams {
		t.TeamIDs = append(t.TeamIDs, tm.ID)
	}
	return p.InTeamScope(t), nil
}

// CheckAssignees refuses, as if they didn't exist, the people the caller
// may not give a task without a project to (RF-6).
func CheckAssignees(ctx context.Context, q *sqlc.Queries, p *authz.Principal, added []uuid.UUID) error {
	for _, id := range added {
		ok, err := CanAssign(ctx, q, p, id)
		if err != nil {
			return err
		}
		if !ok {
			return apperr.ErrNotFound
		}
	}
	return nil
}

// CanTeam: any team not archived for whom may assign to all, the ones a lead
// leads and their subteams (RF-5).
func CanTeam(p *authz.Principal, team sqlc.Team) bool {
	if team.ArchivedAt != nil {
		return false
	}
	return p.Has(authz.LooseAssignAll) || (p.Has(authz.LooseAssignTeam) && p.ScopeTeams[team.ID])
}
