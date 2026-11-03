package authz

import "github.com/google/uuid"

// ProjectRef is what the spec 004 checks need to know about a project.
type ProjectRef struct {
	ID      uuid.UUID
	TeamID  uuid.UUID
	OwnerID uuid.UUID
}

// ProjectRole is a person's role inside one project (spec 008, RF-33 to RF-45).
type ProjectRole string

const (
	ProjectRolePM       ProjectRole = "pm"
	ProjectRoleTechLead ProjectRole = "tech_lead"
	ProjectRoleMember   ProjectRole = "member"
	ProjectRoleObserver ProjectRole = "observer"
)

var ProjectRoles = []ProjectRole{ProjectRolePM, ProjectRoleTechLead, ProjectRoleMember, ProjectRoleObserver}

// WithProjectRoles sets the roles the user was given on purpose.
func (p *Principal) WithProjectRoles(roles map[uuid.UUID]ProjectRole) *Principal {
	for id, r := range roles {
		p.ProjectRoles[id] = r
	}
	return p
}

// ProjectRoleOf is the caller's role in a project: the owner is its PM
// (RF-35), then the role given on purpose, then member for the team's
// participants (RF-36); "" for anyone else.
func (p *Principal) ProjectRoleOf(pr ProjectRef) ProjectRole {
	if pr.OwnerID == p.UserID {
		return ProjectRolePM
	}
	if r, ok := p.ProjectRoles[pr.ID]; ok && pr.ID != uuid.Nil {
		return r
	}
	if p.ParticipantTeams[pr.TeamID] {
		return ProjectRoleMember
	}
	return ""
}

// works reports whether the role works on the project's work items.
func (r ProjectRole) works() bool {
	return r == ProjectRolePM || r == ProjectRoleTechLead || r == ProjectRoleMember
}

// ObjectiveRef is what the checks need to know about an objective. ScopeTeamID
// is the team of the objective (scope equipo) or of its project (scope
// proyecto), and nil for scope organizacion.
type ObjectiveRef struct {
	ScopeTeamID *uuid.UUID
	OwnerID     uuid.UUID
}

// WithParticipantTeams sets the teams the user belongs to plus their ancestors.
func (p *Principal) WithParticipantTeams(ids []uuid.UUID) *Principal {
	for _, id := range ids {
		p.ParticipantTeams[id] = true
	}
	return p
}

// IsParticipant reports whether the user is the project owner or belongs to
// the project team or to any of its subteams.
func (p *Principal) IsParticipant(pr ProjectRef) bool {
	return pr.OwnerID == p.UserID || p.ParticipantTeams[pr.TeamID]
}

// CanCreateProject checks the team the new project would belong to.
func (p *Principal) CanCreateProject(teamID uuid.UUID) bool {
	return p.Has(ProjectsCreateAll) || (p.Has(ProjectsCreateTeam) && p.ScopeTeams[teamID])
}

// CanManageProject: data, lifecycle, phases, enabled types and fields,
// milestones ("gestor" in the spec 004 matrix), and its team, shared views and
// kanban limits (spec 008, RF-25, RF-33, RF-66). A PM manages it like its
// owner while they keep that capability (RF-39).
func (p *Principal) CanManageProject(pr ProjectRef) bool {
	return p.Has(ProjectsManageAll) ||
		(p.Has(ProjectsManageTeam) && p.ScopeTeams[pr.TeamID]) ||
		(p.Has(ProjectsManageOwner) && p.ProjectRoleOf(pr) == ProjectRolePM)
}

// CanWriteWorkItems: create and edit work items, their status, parent and
// dependencies. The global grants come first (RF-43); then a role that works
// on the project, never an observer (RF-40 to RF-42).
func (p *Principal) CanWriteWorkItems(pr ProjectRef) bool {
	return p.Has(WorkItemsWriteAll) ||
		(p.Has(WorkItemsWriteTeam) && p.ScopeTeams[pr.TeamID]) ||
		p.CanManageProject(pr) ||
		(p.Has(WorkItemsWriteParticipant) && p.ProjectRoleOf(pr).works())
}

// CanComment: comment on the work items one can read, viewers excepted
// (spec 006, RF-23). Reading is checked with the project.
func (p *Principal) CanComment() bool { return p.Has(WorkComment) }

// CanDeleteWorkItem: the project managers, or the author while they can still
// write in the project. Whether it has children is checked by the service.
func (p *Principal) CanDeleteWorkItem(pr ProjectRef, authorID uuid.UUID) bool {
	if p.Has(WorkItemsWriteAll) || (p.Has(WorkItemsWriteTeam) && p.ScopeTeams[pr.TeamID]) || p.CanManageProject(pr) {
		return true
	}
	// A tech lead deletes anyone's (spec 008, RF-40).
	if p.Has(WorkItemsDeleteOwn) && p.ProjectRoleOf(pr) == ProjectRoleTechLead {
		return true
	}
	return p.Has(WorkItemsDeleteOwn) && authorID == p.UserID && p.CanWriteWorkItems(pr)
}

// CanCreateObjective checks the scope team of the new objective (nil for organizacion).
func (p *Principal) CanCreateObjective(scopeTeamID *uuid.UUID) bool {
	if p.Has(ObjectivesCreateAll) {
		return true
	}
	return scopeTeamID != nil && p.Has(ObjectivesCreateTeam) && p.ScopeTeams[*scopeTeamID]
}

// CanManageObjective: edit the objective, its key results and their links.
func (p *Principal) CanManageObjective(or ObjectiveRef) bool {
	return p.Has(ObjectivesManageAll) ||
		(p.Has(ObjectivesManageTeam) && or.ScopeTeamID != nil && p.ScopeTeams[*or.ScopeTeamID]) ||
		(p.Has(ObjectivesManageOwner) && or.OwnerID == p.UserID)
}
