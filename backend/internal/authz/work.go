package authz

import "github.com/google/uuid"

// ProjectRef is what the spec 004 checks need to know about a project.
type ProjectRef struct {
	TeamID  uuid.UUID
	OwnerID uuid.UUID
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

// CanManageProject: data, lifecycle, phases, enabled types and fields, and
// milestones ("gestor" in the spec 004 matrix).
func (p *Principal) CanManageProject(pr ProjectRef) bool {
	return p.Has(ProjectsManageAll) ||
		(p.Has(ProjectsManageTeam) && p.ScopeTeams[pr.TeamID]) ||
		(p.Has(ProjectsManageOwner) && pr.OwnerID == p.UserID)
}

// CanWriteWorkItems: create and edit work items, their status, parent and dependencies.
func (p *Principal) CanWriteWorkItems(pr ProjectRef) bool {
	return p.Has(WorkItemsWriteAll) ||
		(p.Has(WorkItemsWriteTeam) && p.ScopeTeams[pr.TeamID]) ||
		(p.Has(WorkItemsWriteParticipant) && p.IsParticipant(pr)) ||
		p.CanManageProject(pr)
}

// CanDeleteWorkItem: the project managers, or the author while they can still
// write in the project. Whether it has children is checked by the service.
func (p *Principal) CanDeleteWorkItem(pr ProjectRef, authorID uuid.UUID) bool {
	if p.Has(WorkItemsWriteAll) || (p.Has(WorkItemsWriteTeam) && p.ScopeTeams[pr.TeamID]) || p.CanManageProject(pr) {
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
