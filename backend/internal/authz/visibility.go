package authz

import "github.com/google/uuid"

// VisibilityProject is what the visibility rule reads from every project.
type VisibilityProject struct {
	ProjectRef
	ParentID  *uuid.UUID
	IsProgram bool
}

// WithVisibility sets the teams and assignments of spec 023 and works out the
// projects the user sees: those of the rule, plus the programs above any of
// them (Conceptos, «Programa visible»).
func (p *Principal) WithVisibility(teams, seenTeams, assigned []uuid.UUID, projects []VisibilityProject) *Principal {
	for _, id := range teams {
		p.Teams[id] = true
	}
	for _, id := range seenTeams {
		p.SeenTeams[id] = true
	}
	for _, id := range assigned {
		p.AssignedProjects[id] = true
	}
	byID := make(map[uuid.UUID]VisibilityProject, len(projects))
	for _, pr := range projects {
		byID[pr.ID] = pr
	}
	for _, pr := range projects {
		if !p.byRule(pr.ProjectRef) {
			continue
		}
		p.visible[pr.ID] = true
		for up := pr.ParentID; up != nil; {
			parent, ok := byID[*up]
			if !ok {
				break
			}
			if parent.IsProgram {
				p.visible[parent.ID], p.viaChild[parent.ID] = true, true
			}
			up = parent.ParentID
		}
	}
	return p
}

// byRule is the rule of spec 023 for one project, without the programs.
func (p *Principal) byRule(pr ProjectRef) bool {
	return p.bySomethingElse(pr) || p.AssignedProjects[pr.ID]
}

// bySomethingElse is the rule without the assigned work items.
func (p *Principal) bySomethingElse(pr ProjectRef) bool {
	return p.Has(ProjectsReadAll) || p.UserID == pr.OwnerID || p.hasProjectRole(pr) ||
		p.SeenTeams[pr.TeamID] || (p.Has(ProjectsReadTeam) && p.ScopeTeams[pr.TeamID])
}

func (p *Principal) hasProjectRole(pr ProjectRef) bool {
	_, ok := p.ProjectRoles[pr.ID]
	return ok && pr.ID != uuid.Nil
}

// SeesWithoutAssignments is SeesProject leaving out the assigned work items,
// which projects.Get reads live instead (RF-16).
func (p *Principal) SeesWithoutAssignments(pr ProjectRef) bool {
	return p.viaChild[pr.ID] || p.bySomethingElse(pr)
}

// SeesProject reports whether the user sees a project that is neither a
// template nor the inbox (spec 023, RF-1). The rule is checked again live, so
// a project just created or moved shows before the principal is reloaded.
func (p *Principal) SeesProject(pr ProjectRef) bool {
	return p.visible[pr.ID] || p.byRule(pr)
}

// SeesAll reports whether the user sees every project, so listings need no filter.
func (p *Principal) SeesAll() bool { return p.Has(ProjectsReadAll) }

// VisibleProjectIDs lists the projects worked out when the principal was
// loaded, for listings filtered in SQL; they add the caller's own projects.
func (p *Principal) VisibleProjectIDs() []uuid.UUID {
	out := make([]uuid.UUID, 0, len(p.visible))
	for id := range p.visible {
		out = append(out, id)
	}
	return out
}

// VisibleTeamIDs are the teams whose projects the user sees: their own,
// ancestors and descendants, and their lead scope. Listings use them so a
// project just created in one of them shows at once.
func (p *Principal) VisibleTeamIDs() []uuid.UUID {
	out := make([]uuid.UUID, 0, len(p.SeenTeams)+len(p.ScopeTeams))
	for id := range p.SeenTeams {
		out = append(out, id)
	}
	if p.Has(ProjectsReadTeam) {
		for id := range p.ScopeTeams {
			if !p.SeenTeams[id] {
				out = append(out, id)
			}
		}
	}
	return out
}

// ReadsByAssignment reports whether a user who sees the project sees it only
// because they have work items in it: read only, but for their own (RF-15).
func (p *Principal) ReadsByAssignment(pr ProjectRef) bool {
	return !p.SeesWithoutAssignments(pr)
}

// OwnsTracking reports whether a project is one of the user's own for
// Seguimiento without filters (RF-21): owner, role, a team of theirs (not an
// ancestor) or work items in it.
func (p *Principal) OwnsTracking(pr ProjectRef) bool {
	return p.UserID == pr.OwnerID || p.hasProjectRole(pr) || p.Teams[pr.TeamID] || p.AssignedProjects[pr.ID]
}
