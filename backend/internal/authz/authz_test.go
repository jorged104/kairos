package authz

import (
	"slices"
	"testing"

	"github.com/google/uuid"
)

func TestRolesInPriorityOrder(t *testing.T) {
	want := []Role{"admin", "lead", "member", "viewer"}
	if !slices.Equal(Roles, want) {
		t.Fatalf("Roles = %v, want %v", Roles, want)
	}
	if got := TopRole([]Role{RoleViewer, RoleLead, RoleMember}); got != RoleLead {
		t.Fatalf("TopRole = %q, want lead", got)
	}
	if got := TopRole(nil); got != "" {
		t.Fatalf("TopRole(nil) = %q, want empty", got)
	}
}

func TestPermissionsAreTheUnionOfRoles(t *testing.T) {
	got := PermissionsFor([]Role{RoleViewer, RoleLead})
	for _, p := range []Permission{DirectoryRead, SelfPassword, UsersUpdateTeam} {
		if !slices.Contains(got, p) {
			t.Errorf("missing %s in %v", p, got)
		}
	}
	if slices.Contains(got, UsersOffboard) {
		t.Errorf("lead+viewer must not offboard")
	}
	if len(PermissionsFor([]Role{RoleMember, RoleMember})) != len(PermissionsFor([]Role{RoleMember})) {
		t.Error("duplicated permissions")
	}
}

func TestMatrixRowsForAdminOnlyPermissions(t *testing.T) {
	adminOnly := []Permission{UsersOffboard, TeamsManage, AttributesManage, IdpMappingsManage, PasswordsSet, AuditRead}
	for _, r := range []Role{RoleLead, RoleMember, RoleViewer} {
		perms := PermissionsFor([]Role{r})
		for _, p := range adminOnly {
			if slices.Contains(perms, p) {
				t.Errorf("%s has admin-only permission %s", r, p)
			}
		}
	}
	for _, r := range Roles {
		perms := PermissionsFor([]Role{r})
		if !slices.Contains(perms, DirectoryRead) || !slices.Contains(perms, SelfPassword) {
			t.Errorf("%s lacks a permission every role has", r)
		}
	}
}

func TestTeamScope(t *testing.T) {
	team, sub, other := uuid.New(), uuid.New(), uuid.New()
	lead := NewPrincipal(uuid.New(), "active", []Role{RoleLead}, []uuid.UUID{team, sub})

	cases := []struct {
		name   string
		target Target
		want   bool
	}{
		{"member in led team", Target{UserID: uuid.New(), Roles: []Role{RoleMember}, TeamIDs: []uuid.UUID{team}}, true},
		{"viewer in subteam", Target{UserID: uuid.New(), Roles: []Role{RoleViewer}, TeamIDs: []uuid.UUID{sub}}, true},
		{"user without role in team", Target{UserID: uuid.New(), TeamIDs: []uuid.UUID{team}}, true},
		{"member outside scope", Target{UserID: uuid.New(), Roles: []Role{RoleMember}, TeamIDs: []uuid.UUID{other}}, false},
		{"lead in team", Target{UserID: uuid.New(), Roles: []Role{RoleLead}, TeamIDs: []uuid.UUID{team}}, false},
		{"admin+member in team", Target{UserID: uuid.New(), Roles: []Role{RoleMember, RoleAdmin}, TeamIDs: []uuid.UUID{team}}, false},
		{"self", Target{UserID: lead.UserID, Roles: []Role{RoleMember}, TeamIDs: []uuid.UUID{team}}, false},
	}
	for _, c := range cases {
		if got := lead.InTeamScope(c.target); got != c.want {
			t.Errorf("%s: InTeamScope = %v, want %v", c.name, got, c.want)
		}
		if got := lead.CanActOn(UsersUpdateAll, UsersUpdateTeam, c.target); got != c.want {
			t.Errorf("%s: CanActOn = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestAssignRole(t *testing.T) {
	team := uuid.New()
	lead := NewPrincipal(uuid.New(), "active", []Role{RoleLead}, []uuid.UUID{team})
	member := Target{UserID: uuid.New(), Roles: []Role{RoleMember}, TeamIDs: []uuid.UUID{team}}

	if !lead.CanAssignRole(RoleViewer, member) {
		t.Error("lead cannot assign viewer in scope")
	}
	if lead.CanAssignRole(RoleLead, member) || lead.CanAssignRole(RoleAdmin, member) {
		t.Error("lead can assign lead/admin (RF-64)")
	}
	if lead.CanAssignRole(RoleMember, Target{UserID: lead.UserID, TeamIDs: []uuid.UUID{team}}) {
		t.Error("lead can change own roles (RF-65)")
	}
	admin := NewPrincipal(uuid.New(), "active", []Role{RoleAdmin}, nil)
	if !admin.CanAssignRole(RoleAdmin, Target{UserID: admin.UserID}) {
		t.Error("admin cannot change own roles")
	}
}

func TestReadFull(t *testing.T) {
	member := NewPrincipal(uuid.New(), "active", []Role{RoleMember}, nil)
	if !member.CanReadFull(Target{UserID: member.UserID}) {
		t.Error("member cannot read own profile")
	}
	if member.CanReadFull(Target{UserID: uuid.New(), Roles: []Role{RoleMember}}) {
		t.Error("member can read someone else's full profile")
	}
}
