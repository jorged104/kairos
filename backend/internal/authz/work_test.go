package authz

import (
	"slices"
	"testing"

	"github.com/google/uuid"
)

func TestWorkPermissionsByRole(t *testing.T) {
	adminOnly := []Permission{CatalogManage, TemplatesManage, ProjectsCreateAll, ProjectsManageAll,
		WorkItemsWriteAll, ObjectivesCreateAll, ObjectivesManageAll}
	for _, r := range []Role{RoleLead, RoleMember, RoleViewer} {
		for _, p := range adminOnly {
			if slices.Contains(PermissionsFor([]Role{r}), p) {
				t.Errorf("%s has admin-only permission %s", r, p)
			}
		}
	}
	for _, r := range Roles {
		if !slices.Contains(PermissionsFor([]Role{r}), WorkRead) {
			t.Errorf("%s cannot read work (spec 004 matrix)", r)
		}
	}
	if got := PermissionsFor([]Role{RoleViewer}); len(got) != 3 {
		t.Errorf("viewer permissions = %v, want only reads and own password", got)
	}
	for _, r := range []Role{RoleAdmin, RoleLead, RoleMember} {
		if !slices.Contains(PermissionsFor([]Role{r}), WorkComment) {
			t.Errorf("%s cannot comment (spec 006, RF-23)", r)
		}
	}
	if slices.Contains(PermissionsFor([]Role{RoleViewer}), WorkComment) {
		t.Error("viewer can comment")
	}
	if slices.Contains(PermissionsFor([]Role{RoleMember}), TemplatesUse) {
		t.Error("member can use templates")
	}
}

func TestProjectChecks(t *testing.T) {
	team, sub, other := uuid.New(), uuid.New(), uuid.New()
	ownerID := uuid.New()
	inScope := ProjectRef{TeamID: sub, OwnerID: ownerID}
	outOfScope := ProjectRef{TeamID: other, OwnerID: ownerID}

	admin := NewPrincipal(uuid.New(), "active", []Role{RoleAdmin}, nil)
	lead := NewPrincipal(uuid.New(), "active", []Role{RoleLead}, []uuid.UUID{team, sub})
	// A member of a subteam of "other" participates in projects of "other".
	member := NewPrincipal(uuid.New(), "active", []Role{RoleMember}, nil).WithParticipantTeams([]uuid.UUID{uuid.New(), other})
	owner := NewPrincipal(ownerID, "active", []Role{RoleMember}, nil)
	viewerOwner := NewPrincipal(ownerID, "active", []Role{RoleViewer}, nil).WithParticipantTeams([]uuid.UUID{other})
	stranger := NewPrincipal(uuid.New(), "active", []Role{RoleMember}, nil)

	cases := []struct {
		name                 string
		p                    *Principal
		pr                   ProjectRef
		create, manage, work bool
	}{
		{"admin", admin, outOfScope, true, true, true},
		{"lead in scope", lead, inScope, true, true, true},
		{"lead out of scope", lead, outOfScope, false, false, false},
		{"participant member", member, outOfScope, false, false, true},
		{"owner member", owner, outOfScope, false, true, true},
		{"viewer owner", viewerOwner, outOfScope, false, false, false},
		{"stranger member", stranger, inScope, false, false, false},
	}
	for _, c := range cases {
		if got := c.p.CanCreateProject(c.pr.TeamID); got != c.create {
			t.Errorf("%s: CanCreateProject = %v, want %v", c.name, got, c.create)
		}
		if got := c.p.CanManageProject(c.pr); got != c.manage {
			t.Errorf("%s: CanManageProject = %v, want %v", c.name, got, c.manage)
		}
		if got := c.p.CanWriteWorkItems(c.pr); got != c.work {
			t.Errorf("%s: CanWriteWorkItems = %v, want %v", c.name, got, c.work)
		}
	}

	// Deleting: managers always; a member only what they authored.
	if !member.CanDeleteWorkItem(outOfScope, member.UserID) {
		t.Error("participant cannot delete own work item")
	}
	if member.CanDeleteWorkItem(outOfScope, uuid.New()) {
		t.Error("participant can delete someone else's work item")
	}
	if !owner.CanDeleteWorkItem(outOfScope, uuid.New()) {
		t.Error("owner cannot delete a work item of the project")
	}
	if stranger.CanDeleteWorkItem(inScope, stranger.UserID) {
		t.Error("an author who no longer participates can still delete")
	}
}

func TestObjectiveChecks(t *testing.T) {
	team, other := uuid.New(), uuid.New()
	ownerID := uuid.New()
	admin := NewPrincipal(uuid.New(), "active", []Role{RoleAdmin}, nil)
	lead := NewPrincipal(uuid.New(), "active", []Role{RoleLead}, []uuid.UUID{team})
	owner := NewPrincipal(ownerID, "active", []Role{RoleMember}, nil)
	viewerOwner := NewPrincipal(ownerID, "active", []Role{RoleViewer}, nil)

	if !admin.CanCreateObjective(nil) || lead.CanCreateObjective(nil) {
		t.Error("only admin creates organization objectives")
	}
	if !lead.CanCreateObjective(&team) || lead.CanCreateObjective(&other) {
		t.Error("lead creates team objectives only in scope")
	}
	if owner.CanCreateObjective(&team) {
		t.Error("member creates objectives")
	}

	inTeam := ObjectiveRef{ScopeTeamID: &team, OwnerID: ownerID}
	org := ObjectiveRef{OwnerID: ownerID}
	if !lead.CanManageObjective(inTeam) || lead.CanManageObjective(org) {
		t.Error("lead manages only objectives of its scope")
	}
	if !owner.CanManageObjective(org) || viewerOwner.CanManageObjective(org) {
		t.Error("owner manages own objective unless viewer")
	}
	if !admin.CanManageObjective(org) {
		t.Error("admin cannot manage objective")
	}
}
