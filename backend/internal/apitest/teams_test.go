package apitest

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

type teamDTO struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	ParentID    *uuid.UUID `json:"parent_id"`
	ExternalRef *string    `json:"external_ref"`
	ArchivedAt  *string    `json:"archived_at"`
	Leaders     []struct {
		UserID uuid.UUID `json:"user_id"`
	} `json:"leaders"`
}

func createTeam(t *testing.T, c *Client, body map[string]any) teamDTO {
	t.Helper()
	var team teamDTO
	c.Post("/api/teams", body).Expect(t, http.StatusCreated).JSON(t, &team)
	return team
}

func TestCreateTeamsWithHierarchy(t *testing.T) {
	env := New(t)
	admin := env.As(env.CreateUser(UserSpec{Email: "admin@x.com", Roles: []string{"admin"}}))

	bi := createTeam(t, admin, map[string]any{"name": " BI ", "external_ref": "/Areas/BI"})
	if bi.Name != "BI" || bi.ExternalRef == nil || *bi.ExternalRef != "/Areas/BI" {
		t.Fatalf("team = %+v", bi)
	}
	rep := createTeam(t, admin, map[string]any{"name": "BI Reportes", "parent_id": bi.ID})
	if rep.ParentID == nil || *rep.ParentID != bi.ID {
		t.Fatalf("subteam = %+v", rep)
	}
	admin.Post("/api/teams", map[string]any{"name": "bi"}).Expect(t, http.StatusConflict, "team_name_conflict")
	admin.Post("/api/teams", map[string]any{"name": ""}).Expect(t, http.StatusBadRequest, "validation_failed")
	admin.Post("/api/teams", map[string]any{"name": "X", "parent_id": uuid.New()}).Expect(t, http.StatusBadRequest, "validation_failed")

	var list struct{ Items []teamDTO }
	admin.Get("/api/teams").Expect(t, http.StatusOK).JSON(t, &list)
	if len(list.Items) != 2 {
		t.Fatalf("list = %+v", list)
	}
	if got := env.AuditActions(bi.ID.String()); len(got) != 1 || got[0] != "team.created" {
		t.Fatalf("audit = %v", got)
	}
}

func TestTeamCyclesAreRejected(t *testing.T) {
	env := New(t)
	admin := env.As(env.CreateUser(UserSpec{Email: "admin@x.com", Roles: []string{"admin"}}))
	a := createTeam(t, admin, map[string]any{"name": "A"})
	b := createTeam(t, admin, map[string]any{"name": "B", "parent_id": a.ID})
	c := createTeam(t, admin, map[string]any{"name": "C", "parent_id": b.ID})

	admin.Do(http.MethodPatch, "/api/teams/"+a.ID.String(), map[string]any{"name": "A", "parent_id": c.ID}).
		Expect(t, http.StatusConflict, "team_cycle")
	admin.Do(http.MethodPatch, "/api/teams/"+a.ID.String(), map[string]any{"name": "A", "parent_id": a.ID}).
		Expect(t, http.StatusConflict, "team_cycle")
	// Moving C to the root and renaming it works.
	var moved teamDTO
	admin.Do(http.MethodPatch, "/api/teams/"+c.ID.String(), map[string]any{"name": "C2"}).Expect(t, http.StatusOK).JSON(t, &moved)
	if moved.ParentID != nil || moved.Name != "C2" {
		t.Fatalf("moved = %+v", moved)
	}
}

func TestArchiveTeams(t *testing.T) {
	env := New(t)
	admin := env.As(env.CreateUser(UserSpec{Email: "admin@x.com", Roles: []string{"admin"}}))
	parent := createTeam(t, admin, map[string]any{"name": "P"})
	child := createTeam(t, admin, map[string]any{"name": "C", "parent_id": parent.ID})
	member := env.CreateUser(UserSpec{Email: "m@x.com", Roles: []string{"member"}, Teams: []uuid.UUID{child.ID}})

	admin.Post("/api/teams/"+parent.ID.String()+"/archive", nil).Expect(t, http.StatusConflict, "team_has_children")
	admin.Post("/api/teams/"+child.ID.String()+"/archive", nil).Expect(t, http.StatusOK)
	admin.Post("/api/teams/"+parent.ID.String()+"/archive", nil).Expect(t, http.StatusOK)

	// Archived teams can't be parents or be restored under an archived parent.
	admin.Post("/api/teams", map[string]any{"name": "N", "parent_id": parent.ID}).Expect(t, http.StatusConflict, "team_archived")
	admin.Post("/api/teams/"+child.ID.String()+"/unarchive", nil).Expect(t, http.StatusConflict, "team_archived")
	admin.Post("/api/teams/"+parent.ID.String()+"/unarchive", nil).Expect(t, http.StatusOK)

	// Historical memberships are kept (RF-92).
	if n, _ := env.Store.ListUserTeams(t.Context(), member); len(n) != 1 {
		t.Fatalf("membership lost: %v", n)
	}
}

func TestTeamLeadersNeedLeadRole(t *testing.T) {
	env := New(t)
	admin := env.As(env.CreateUser(UserSpec{Email: "admin@x.com", Roles: []string{"admin"}}))
	team := createTeam(t, admin, map[string]any{"name": "BI"})
	lead := env.CreateUser(UserSpec{Email: "lead@x.com", Roles: []string{"lead"}})
	member := env.CreateUser(UserSpec{Email: "m@x.com", Roles: []string{"member"}})
	path := "/api/teams/" + team.ID.String() + "/leaders"

	admin.Post(path, map[string]any{"user_id": member}).Expect(t, http.StatusConflict, "leader_not_lead")
	admin.Post(path, map[string]any{"user_id": lead}).Expect(t, http.StatusNoContent)

	var list struct{ Items []teamDTO }
	admin.Get("/api/teams").JSON(t, &list)
	if len(list.Items[0].Leaders) != 1 || list.Items[0].Leaders[0].UserID != lead {
		t.Fatalf("leaders = %+v", list.Items[0].Leaders)
	}
	admin.Do(http.MethodDelete, path+"/"+lead.String(), nil).Expect(t, http.StatusNoContent)
	admin.Do(http.MethodDelete, path+"/"+lead.String(), nil).Expect(t, http.StatusNotFound)
}

func TestTeamManagementIsAdminOnly(t *testing.T) {
	env := New(t)
	for _, role := range []string{"lead", "member", "viewer"} {
		c := env.As(env.CreateUser(UserSpec{Email: role + "@x.com", Roles: []string{role}}))
		c.Get("/api/teams").Expect(t, http.StatusOK)
		c.Post("/api/teams", map[string]any{"name": "T"}).Expect(t, http.StatusForbidden, "forbidden")
	}
}
