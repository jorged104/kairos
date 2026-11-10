package authz

import (
	"slices"
	"testing"

	"github.com/google/uuid"
)

func TestOversightAndReadPermissions(t *testing.T) {
	for r, want := range map[Role]bool{RoleAdmin: true, RoleDirector: true, RoleLead: true, RoleMember: false, RoleViewer: false} {
		if got := slices.Contains(PermissionsFor([]Role{r}), OversightView); got != want {
			t.Errorf("%s oversight = %v, want %v (spec 023, RF-17a)", r, got, want)
		}
	}
	for r, want := range map[Role]bool{RoleAdmin: true, RoleDirector: true, RoleLead: false, RoleMember: false, RoleViewer: false} {
		if got := slices.Contains(PermissionsFor([]Role{r}), ProjectsReadAll); got != want {
			t.Errorf("%s reads all = %v, want %v (RF-31)", r, got, want)
		}
	}
	director := PermissionsFor([]Role{RoleDirector})
	for _, p := range []Permission{DashboardOthersAll, WorkComment, ProjectViewsShare, ProjectsManageOwner, WorkItemsWriteParticipant} {
		if !slices.Contains(director, p) {
			t.Errorf("director lacks %s (RF-7, RF-8)", p)
		}
	}
	for _, p := range []Permission{UsersCreateAll, UsersUpdateAll, RolesAssignAll, CatalogManage, PortfolioManage, ProjectsManageAll, ProjectsCreateAll} {
		if slices.Contains(director, p) {
			t.Errorf("director has %s (RF-9)", p)
		}
	}
}

func TestSeesProject(t *testing.T) {
	// root > dev > back; other is unrelated.
	root, dev, back, front, other := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	ownerID := uuid.New()
	program := VisibilityProject{ProjectRef: ProjectRef{ID: uuid.New(), TeamID: other, OwnerID: ownerID}, IsProgram: true}
	inOther := VisibilityProject{ProjectRef: ProjectRef{ID: uuid.New(), TeamID: other, OwnerID: ownerID}, ParentID: &program.ID}
	inRoot := VisibilityProject{ProjectRef: ProjectRef{ID: uuid.New(), TeamID: root, OwnerID: ownerID}}
	inFront := VisibilityProject{ProjectRef: ProjectRef{ID: uuid.New(), TeamID: front, OwnerID: ownerID}}
	assigned := VisibilityProject{ProjectRef: ProjectRef{ID: uuid.New(), TeamID: other, OwnerID: ownerID}, ParentID: &program.ID}
	all := []VisibilityProject{program, inOther, inRoot, inFront, assigned}

	// A member of dev sees its ancestors' and descendants' projects, not a sibling's.
	member := NewPrincipal(uuid.New(), "active", []Role{RoleMember}, nil).
		WithVisibility([]uuid.UUID{dev}, []uuid.UUID{dev, root, back}, []uuid.UUID{assigned.ID}, all)
	for name, c := range map[string]struct {
		pr   ProjectRef
		want bool
	}{
		"ancestor team":            {inRoot.ProjectRef, true},
		"sibling team":             {inFront.ProjectRef, false},
		"unrelated team":           {inOther.ProjectRef, false},
		"assigned work item":       {assigned.ProjectRef, true},
		"program above a visible":  {program.ProjectRef, true},
		"owner of a new project":   {ProjectRef{ID: uuid.New(), TeamID: other, OwnerID: member.UserID}, true},
		"descendant team, created": {ProjectRef{ID: uuid.New(), TeamID: back, OwnerID: ownerID}, true},
	} {
		if got := member.SeesProject(c.pr); got != c.want {
			t.Errorf("%s: SeesProject = %v, want %v", name, got, c.want)
		}
	}
	if !member.ReadsByAssignment(assigned.ProjectRef) || member.ReadsByAssignment(inRoot.ProjectRef) {
		t.Error("ReadsByAssignment (RF-15)")
	}
	if !member.OwnsTracking(assigned.ProjectRef) || member.OwnsTracking(inRoot.ProjectRef) {
		t.Error("OwnsTracking counts direct teams only (RF-21)")
	}

	// A lead sees their scope; a role in a project shows it; director sees all.
	lead := NewPrincipal(uuid.New(), "active", []Role{RoleLead}, []uuid.UUID{front}).WithVisibility(nil, nil, nil, all)
	observer := NewPrincipal(uuid.New(), "active", []Role{RoleViewer}, nil).
		WithProjectRoles(map[uuid.UUID]ProjectRole{inOther.ID: ProjectRoleObserver}).WithVisibility(nil, nil, nil, all)
	director := NewPrincipal(uuid.New(), "active", []Role{RoleDirector}, nil).WithVisibility(nil, nil, nil, all)
	if !lead.SeesProject(inFront.ProjectRef) || lead.SeesProject(inRoot.ProjectRef) {
		t.Error("lead scope")
	}
	if !observer.SeesProject(inOther.ProjectRef) || observer.SeesProject(inRoot.ProjectRef) {
		t.Error("project role")
	}
	if !director.SeesProject(inOther.ProjectRef) || !director.SeesAll() {
		t.Error("director")
	}
	if got := len(member.VisibleProjectIDs()); got != 3 {
		t.Errorf("member visible = %d, want 3", got)
	}
}
