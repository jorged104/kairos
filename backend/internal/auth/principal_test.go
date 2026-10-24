package auth

import (
	"context"
	"testing"
	"time"

	"openscum/internal/authz"
	"openscum/internal/db"
	"openscum/internal/db/dbtest"
	"openscum/internal/db/sqlc"
)

func TestPrincipalLoaderCachesUntilTTLOrInvalidate(t *testing.T) {
	ctx := context.Background()
	store := db.NewStore(dbtest.New(t))
	u, err := store.CreateUser(ctx, sqlc.CreateUserParams{Name: "A", Email: "a@x.com", EmailNormalized: "a@x.com", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddManualRole(ctx, sqlc.AddManualRoleParams{UserID: u.ID, Role: "member"}); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	l := NewPrincipalLoader(store, PrincipalTTL)
	l.now = func() time.Time { return now }

	p, err := l.Load(ctx, u.ID)
	if err != nil || !p.HasRole(authz.RoleMember) {
		t.Fatalf("Load = %+v, %v", p, err)
	}
	if err := store.AddManualRole(ctx, sqlc.AddManualRoleParams{UserID: u.ID, Role: "admin"}); err != nil {
		t.Fatal(err)
	}

	now = now.Add(PrincipalTTL - time.Second)
	if p, _ := l.Load(ctx, u.ID); p.HasRole(authz.RoleAdmin) {
		t.Fatal("cache refreshed before the TTL")
	}
	now = now.Add(2 * time.Second)
	if p, _ := l.Load(ctx, u.ID); !p.HasRole(authz.RoleAdmin) {
		t.Fatal("cache not refreshed after the TTL")
	}

	if _, err := store.SetUserStatus(ctx, sqlc.SetUserStatusParams{ID: u.ID, Status: "inactive"}); err != nil {
		t.Fatal(err)
	}
	l.Invalidate(u.ID)
	if p, _ := l.Load(ctx, u.ID); p.Status != "inactive" {
		t.Fatalf("status = %q after Invalidate, want inactive", p.Status)
	}
}

func TestPrincipalTTLWithinSpecWindow(t *testing.T) {
	if PrincipalTTL > 120*time.Second {
		t.Fatalf("PrincipalTTL = %v, RF-44 allows at most 120 s", PrincipalTTL)
	}
}

func TestPrincipalScopeIncludesSubteams(t *testing.T) {
	ctx := context.Background()
	store := db.NewStore(dbtest.New(t))
	lead, _ := store.CreateUser(ctx, sqlc.CreateUserParams{Name: "L", Email: "l@x.com", EmailNormalized: "l@x.com", Status: "active"})
	parent, err := store.CreateTeam(ctx, sqlc.CreateTeamParams{Name: "BI", NameNormalized: "bi"})
	if err != nil {
		t.Fatal(err)
	}
	child, _ := store.CreateTeam(ctx, sqlc.CreateTeamParams{Name: "BI Rep", NameNormalized: "bi rep", ParentID: &parent.ID})
	grandchild, _ := store.CreateTeam(ctx, sqlc.CreateTeamParams{Name: "BI Rep 2", NameNormalized: "bi rep 2", ParentID: &child.ID})
	other, _ := store.CreateTeam(ctx, sqlc.CreateTeamParams{Name: "Otro", NameNormalized: "otro"})
	if err := store.AddTeamLeader(ctx, sqlc.AddTeamLeaderParams{TeamID: parent.ID, UserID: lead.ID}); err != nil {
		t.Fatal(err)
	}

	p, err := NewPrincipalLoader(store, PrincipalTTL).Load(ctx, lead.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !p.ScopeTeams[parent.ID] || !p.ScopeTeams[child.ID] || !p.ScopeTeams[grandchild.ID] || p.ScopeTeams[other.ID] {
		t.Fatalf("scope = %v", p.ScopeTeams)
	}
}

func TestPrincipalParticipantTeamsIncludeAncestors(t *testing.T) {
	ctx := context.Background()
	store := db.NewStore(dbtest.New(t))
	u, _ := store.CreateUser(ctx, sqlc.CreateUserParams{Name: "M", Email: "m@x.com", EmailNormalized: "m@x.com", Status: "active"})
	parent, _ := store.CreateTeam(ctx, sqlc.CreateTeamParams{Name: "BI", NameNormalized: "bi"})
	child, _ := store.CreateTeam(ctx, sqlc.CreateTeamParams{Name: "BI Rep", NameNormalized: "bi rep", ParentID: &parent.ID})
	grandchild, _ := store.CreateTeam(ctx, sqlc.CreateTeamParams{Name: "BI Rep 2", NameNormalized: "bi rep 2", ParentID: &child.ID})
	if err := store.AddManualTeamMember(ctx, sqlc.AddManualTeamMemberParams{TeamID: child.ID, UserID: u.ID}); err != nil {
		t.Fatal(err)
	}

	p, err := NewPrincipalLoader(store, PrincipalTTL).Load(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !p.ParticipantTeams[parent.ID] || !p.ParticipantTeams[child.ID] || p.ParticipantTeams[grandchild.ID] {
		t.Fatalf("participant teams = %v, want the team and its ancestors only", p.ParticipantTeams)
	}
}
