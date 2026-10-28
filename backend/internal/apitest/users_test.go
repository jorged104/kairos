package apitest

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"openscum/internal/config"
	"openscum/internal/db/sqlc"
)

type userDTO struct {
	ID          uuid.UUID                       `json:"id"`
	Name        string                          `json:"name"`
	Status      string                          `json:"status"`
	FullProfile bool                            `json:"full_profile"`
	Email       *string                         `json:"email"`
	Roles       []struct{ Role, Source string } `json:"roles"`
	Teams       []struct {
		ID     uuid.UUID `json:"id"`
		Source string    `json:"source"`
	} `json:"teams"`
	Attributes        map[uuid.UUID]json.RawMessage `json:"attributes"`
	RetiredAttributes map[uuid.UUID]json.RawMessage `json:"retired_attributes"`
	HasPassword       *bool                         `json:"has_password"`
	Can               *struct {
		Edit bool `json:"edit"`
	} `json:"can"`
}

func (u userDTO) role(name string) string {
	for _, r := range u.Roles {
		if r.Role == name {
			return r.Source
		}
	}
	return ""
}

func (u userDTO) inTeam(id uuid.UUID) bool {
	return slices.ContainsFunc(u.Teams, func(t struct {
		ID     uuid.UUID `json:"id"`
		Source string    `json:"source"`
	}) bool {
		return t.ID == id
	})
}

func decodeUser(t *testing.T, r *Response) userDTO {
	t.Helper()
	var u userDTO
	r.JSON(t, &u)
	return u
}

// fixture: an admin, a lead of BI, a member in the subteam BI Reportes and an outsider.
type fixture struct {
	env                                    *Env
	admin, lead, member, outsider, otherLd uuid.UUID
	bi, biRep, other                       uuid.UUID
}

func newFixture(t *testing.T, opts ...Option) fixture {
	env := New(t, opts...)
	f := fixture{env: env}
	f.admin = env.CreateUser(UserSpec{Name: "Admin", Email: "admin@x.com", Roles: []string{"admin"}, Password: pw})
	f.lead = env.CreateUser(UserSpec{Name: "Lead", Email: "lead@x.com", Roles: []string{"lead"}, Password: pw})
	f.bi = env.CreateTeam("BI", nil, f.lead)
	f.biRep = env.CreateTeam("BI Reportes", &f.bi)
	f.other = env.CreateTeam("Otro", nil)
	f.member = env.CreateUser(UserSpec{Name: "Member", Email: "member@x.com", Roles: []string{"member"}, Teams: []uuid.UUID{f.biRep}, Password: pw})
	f.otherLd = env.CreateUser(UserSpec{Name: "Otro lead", Email: "lead2@x.com", Roles: []string{"lead"}, Teams: []uuid.UUID{f.bi}})
	f.outsider = env.CreateUser(UserSpec{Name: "Outsider", Email: "out@x.com", Roles: []string{"member"}, Teams: []uuid.UUID{f.other}})
	return f
}

func TestAdminCreatesUser(t *testing.T) {
	f := newFixture(t)
	admin := f.env.As(f.admin)
	u := decodeUser(t, admin.Post("/api/users", map[string]any{
		"name": "Nuevo", "email": " Nuevo@X.com ", "password": "contrasena-inicial", "team_id": f.bi,
	}).Expect(t, http.StatusCreated))
	if u.Status != "active" || u.role("member") != "manual" || !u.inTeam(f.bi) || !*u.HasPassword {
		t.Fatalf("created = %+v", u)
	}
	login(f.env.Client(), "nuevo@x.com", "contrasena-inicial").Expect(t, http.StatusOK) // RF-69
	if got := f.env.AuditActions(u.ID.String()); !slices.Contains(got, "user.created") || !slices.Contains(got, "user.team_added") {
		t.Fatalf("audit = %v", got)
	}

	// Duplicate emails are rejected, even of an offboarded user (RF-71).
	admin.Post("/api/users", map[string]any{"name": "X", "email": "NUEVO@x.com"}).Expect(t, http.StatusConflict, "email_conflict")
	admin.Post("/api/users/"+u.ID.String()+"/offboard", nil).Expect(t, http.StatusOK)
	admin.Post("/api/users", map[string]any{"name": "X", "email": "nuevo@x.com"}).Expect(t, http.StatusConflict, "email_conflict")
}

func TestLeadCreatesUsersOnlyInItsTeams(t *testing.T) {
	f := newFixture(t)
	lead := f.env.As(f.lead)
	u := decodeUser(t, lead.Post("/api/users", map[string]any{"name": "N", "email": "n@x.com", "team_id": f.biRep, "role": "viewer"}).
		Expect(t, http.StatusCreated))
	if !u.inTeam(f.biRep) || u.role("viewer") != "manual" {
		t.Fatalf("created = %+v", u)
	}
	lead.Post("/api/users", map[string]any{"name": "N", "email": "n2@x.com"}).Expect(t, http.StatusBadRequest, "validation_failed")
	lead.Post("/api/users", map[string]any{"name": "N", "email": "n3@x.com", "team_id": f.other}).Expect(t, http.StatusForbidden, "forbidden")
	lead.Post("/api/users", map[string]any{"name": "N", "email": "n4@x.com", "team_id": f.bi, "role": "lead"}).Expect(t, http.StatusForbidden, "forbidden")
	lead.Post("/api/users", map[string]any{"name": "N", "email": "n5@x.com", "team_id": f.bi, "password": "contrasena-inicial"}).Expect(t, http.StatusForbidden, "forbidden")
	f.env.As(f.member).Post("/api/users", map[string]any{"name": "N", "email": "n6@x.com"}).Expect(t, http.StatusForbidden, "forbidden")
}

func TestCreateAndEditCheckAttributes(t *testing.T) {
	f := newFixture(t)
	admin := f.env.As(f.admin)
	ingreso := createAttr(t, admin, map[string]any{"name": "Fecha de ingreso", "type": "date", "required": true})

	res := admin.Post("/api/users", map[string]any{"name": "N", "email": "n@x.com"}).Expect(t, http.StatusBadRequest, "attribute_required")
	if !strings.Contains(string(res.Body), ingreso.ID.String()) {
		t.Fatalf("error does not name the attribute: %s", res.Body)
	}
	admin.Post("/api/users", map[string]any{"name": "N", "email": "n@x.com", "attributes": map[string]any{ingreso.ID.String(): "abc"}}).
		Expect(t, http.StatusBadRequest, "attribute_invalid_value")
	u := decodeUser(t, admin.Post("/api/users", map[string]any{"name": "N", "email": "n@x.com", "attributes": map[string]any{ingreso.ID.String(): "2026-10-01"}}).
		Expect(t, http.StatusCreated))
	if string(u.Attributes[ingreso.ID]) != `"2026-10-01"` {
		t.Fatalf("attributes = %v", u.Attributes)
	}

	// Existing users without the value are not invalid, but their next edit asks for it (RF-99).
	f.env.As(f.member).Get("/api/users/"+f.member.String()).Expect(t, http.StatusOK)
	lead := f.env.As(f.lead)
	lead.Do(http.MethodPatch, "/api/users/"+f.member.String(), map[string]any{"name": "Miembro"}).Expect(t, http.StatusBadRequest, "attribute_required")
	m := decodeUser(t, lead.Do(http.MethodPatch, "/api/users/"+f.member.String(), map[string]any{
		"name": "Miembro", "attributes": map[string]any{ingreso.ID.String(): "2025-01-01"},
	}).Expect(t, http.StatusOK))
	if m.Name != "Miembro" {
		t.Fatalf("edited = %+v", m)
	}
}

func TestEditScopeAndConflicts(t *testing.T) {
	f := newFixture(t)
	lead := f.env.As(f.lead)
	lead.Do(http.MethodPatch, "/api/users/"+f.member.String(), map[string]any{"name": "M2"}).Expect(t, http.StatusOK)
	lead.Do(http.MethodPatch, "/api/users/"+f.otherLd.String(), map[string]any{"name": "X"}).Expect(t, http.StatusForbidden, "forbidden")
	lead.Do(http.MethodPatch, "/api/users/"+f.outsider.String(), map[string]any{"name": "X"}).Expect(t, http.StatusForbidden, "forbidden")
	lead.Do(http.MethodPatch, "/api/users/"+f.member.String(), map[string]any{"email": "OUT@x.com"}).Expect(t, http.StatusConflict, "email_conflict")
	f.env.As(f.member).Do(http.MethodPatch, "/api/users/"+f.member.String(), map[string]any{"name": "Yo"}).Expect(t, http.StatusForbidden, "forbidden")
}

func TestDeactivateAndReactivateKeepEverything(t *testing.T) {
	f := newFixture(t)
	lead := f.env.As(f.lead)
	path := "/api/users/" + f.member.String()
	u := decodeUser(t, lead.Post(path+"/deactivate", nil).Expect(t, http.StatusOK))
	if u.Status != "inactive" || u.role("member") == "" || !u.inTeam(f.biRep) {
		t.Fatalf("deactivated = %+v", u)
	}
	login(f.env.Client(), "member@x.com", pw).Expect(t, http.StatusForbidden, "account_inactive")
	f.env.As(f.member).Get("/api/me").Expect(t, http.StatusForbidden, "account_inactive")

	lead.Post(path+"/reactivate", nil).Expect(t, http.StatusOK)
	var me struct {
		Roles []struct{ Role string }
	}
	login(f.env.Client(), "member@x.com", pw).Expect(t, http.StatusOK).JSON(t, &me)
	if len(me.Roles) != 1 || me.Roles[0].Role != "member" {
		t.Fatalf("roles after reactivation = %+v", me.Roles)
	}
	if got := f.env.AuditActions(f.member.String()); len(got) != 2 || got[0] != "user.status_changed" {
		t.Fatalf("audit = %v", got)
	}
	lead.Post("/api/users/"+f.otherLd.String()+"/deactivate", nil).Expect(t, http.StatusForbidden, "forbidden") // RF-63
}

func TestOffboardAndRestore(t *testing.T) {
	f := newFixture(t)
	admin, lead := f.env.As(f.admin), f.env.As(f.lead)
	path := "/api/users/" + f.member.String()

	lead.Post(path+"/offboard", nil).Expect(t, http.StatusForbidden, "forbidden")
	u := decodeUser(t, admin.Post(path+"/offboard", nil).Expect(t, http.StatusOK))
	if u.Status != "offboarded" || u.role("member") == "" {
		t.Fatalf("offboarded = %+v", u)
	}
	login(f.env.Client(), "member@x.com", pw).Expect(t, http.StatusForbidden, "account_inactive")
	lead.Post(path+"/reactivate", nil).Expect(t, http.StatusForbidden, "forbidden") // RF-78a
	admin.Post(path+"/reactivate", nil).Expect(t, http.StatusConflict, "user_offboarded")

	u = decodeUser(t, admin.Post(path+"/restore", nil).Expect(t, http.StatusOK))
	if u.Status != "inactive" || !u.inTeam(f.biRep) { // RF-78
		t.Fatalf("restored = %+v", u)
	}
	admin.Post(path+"/reactivate", nil).Expect(t, http.StatusOK)
}

func TestLastAdminIsKept(t *testing.T) {
	f := newFixture(t)
	admin := f.env.As(f.admin)
	self := "/api/users/" + f.admin.String()
	admin.Do(http.MethodDelete, self+"/roles/admin", nil).Expect(t, http.StatusConflict, "last_admin")
	admin.Post(self+"/deactivate", nil).Expect(t, http.StatusConflict, "last_admin")
	admin.Post(self+"/offboard", nil).Expect(t, http.StatusConflict, "last_admin")

	other := f.env.CreateUser(UserSpec{Email: "admin2@x.com", Roles: []string{"admin"}})
	admin.Do(http.MethodDelete, self+"/roles/admin", nil).Expect(t, http.StatusOK)
	f.env.As(other).Post("/api/users/"+other.String()+"/deactivate", nil).Expect(t, http.StatusConflict, "last_admin")
}

func TestAdminSetsPasswordWithAudit(t *testing.T) {
	f := newFixture(t)
	path := "/api/users/" + f.member.String() + "/password"
	f.env.As(f.lead).Do(http.MethodPut, path, map[string]any{"password": "otra-contrasena-1"}).Expect(t, http.StatusForbidden, "forbidden")
	f.env.As(f.admin).Do(http.MethodPut, path, map[string]any{"password": "otra-contrasena-1"}).Expect(t, http.StatusNoContent)
	login(f.env.Client(), "member@x.com", "otra-contrasena-1").Expect(t, http.StatusOK)

	var raw string
	if err := f.env.Store.Pool.QueryRow(context.Background(),
		"SELECT coalesce(before::text,'') || coalesce(after::text,'') || action FROM audit_log WHERE action = 'user.password_set'").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "otra-contrasena-1") {
		t.Fatal("audit contains the password")
	}
}

func TestRoleAssignment(t *testing.T) {
	f := newFixture(t)
	lead, admin := f.env.As(f.lead), f.env.As(f.admin)
	path := "/api/users/" + f.member.String() + "/roles"

	u := decodeUser(t, lead.Post(path, map[string]any{"role": "viewer"}).Expect(t, http.StatusOK))
	if u.role("viewer") != "manual" {
		t.Fatalf("roles = %+v", u.Roles)
	}
	lead.Post(path, map[string]any{"role": "lead"}).Expect(t, http.StatusForbidden, "forbidden")  // RF-64
	lead.Post(path, map[string]any{"role": "admin"}).Expect(t, http.StatusForbidden, "forbidden") // RF-64
	lead.Do(http.MethodDelete, path+"/viewer", nil).Expect(t, http.StatusOK)
	lead.Do(http.MethodDelete, path+"/viewer", nil).Expect(t, http.StatusNotFound)
	f.env.As(f.member).Post(path, map[string]any{"role": "viewer"}).Expect(t, http.StatusForbidden, "forbidden") // RF-65
	lead.Post("/api/users/"+f.lead.String()+"/roles", map[string]any{"role": "viewer"}).Expect(t, http.StatusForbidden, "forbidden")
	admin.Post("/api/users/"+f.admin.String()+"/roles", map[string]any{"role": "lead"}).Expect(t, http.StatusOK)
	if got := f.env.AuditActions(f.member.String()); !slices.Contains(got, "user.role_removed") {
		t.Fatalf("audit = %v", got)
	}
}

func TestIdPAssignmentsCannotBeRemovedByHand(t *testing.T) {
	f := newFixture(t, func(c *config.Config) { c.OIDC.RoleSource = config.RoleSourceIdP })
	ctx := context.Background()
	if _, err := f.env.Store.AddIdpRole(ctx, sqlc.AddIdpRoleParams{UserID: f.member, Role: "viewer"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.env.Store.AddIdpTeamMember(ctx, sqlc.AddIdpTeamMemberParams{TeamID: f.bi, UserID: f.member}); err != nil {
		t.Fatal(err)
	}
	admin := f.env.As(f.admin)
	u := decodeUser(t, admin.Get("/api/users/"+f.member.String()))
	if u.role("viewer") != "idp" { // RF-86
		t.Fatalf("roles = %+v", u.Roles)
	}
	admin.Do(http.MethodDelete, "/api/users/"+f.member.String()+"/roles/viewer", nil).Expect(t, http.StatusConflict, "idp_managed")
	admin.Do(http.MethodDelete, "/api/users/"+f.member.String()+"/teams/"+f.bi.String(), nil).Expect(t, http.StatusConflict, "idp_managed")
}

func TestIdPRolesRemovableWhenRolesAreLocal(t *testing.T) {
	f := newFixture(t, func(c *config.Config) { c.OIDC.RoleSource = config.RoleSourceLocal })
	if _, err := f.env.Store.AddIdpRole(context.Background(), sqlc.AddIdpRoleParams{UserID: f.member, Role: "viewer"}); err != nil {
		t.Fatal(err)
	}
	f.env.As(f.admin).Do(http.MethodDelete, "/api/users/"+f.member.String()+"/roles/viewer", nil).Expect(t, http.StatusOK)
}

func TestTeamMembership(t *testing.T) {
	f := newFixture(t)
	lead, admin := f.env.As(f.lead), f.env.As(f.admin)
	path := "/api/users/" + f.member.String() + "/teams"

	u := decodeUser(t, lead.Post(path, map[string]any{"team_id": f.bi}).Expect(t, http.StatusOK))
	if !u.inTeam(f.bi) || !u.inTeam(f.biRep) { // RF-87
		t.Fatalf("teams = %+v", u.Teams)
	}
	lead.Post(path, map[string]any{"team_id": f.other}).Expect(t, http.StatusForbidden, "forbidden")
	lead.Post("/api/users/"+f.outsider.String()+"/teams", map[string]any{"team_id": f.bi}).Expect(t, http.StatusForbidden, "forbidden")
	admin.Post(path, map[string]any{"team_id": f.other}).Expect(t, http.StatusOK)
	lead.Do(http.MethodDelete, path+"/"+f.bi.String(), nil).Expect(t, http.StatusOK)

	archived := f.env.CreateTeam("Viejo", nil)
	admin.Post("/api/teams/"+archived.String()+"/archive", nil).Expect(t, http.StatusOK)
	admin.Post(path, map[string]any{"team_id": archived}).Expect(t, http.StatusConflict, "team_archived")
}

func TestProfileVisibility(t *testing.T) {
	f := newFixture(t)
	admin := f.env.As(f.admin)
	salida := createAttr(t, admin, map[string]any{"name": "Salida", "type": "date"})
	admin.Do(http.MethodPatch, "/api/users/"+f.member.String(), map[string]any{"attributes": map[string]any{salida.ID.String(): "2026-12-31"}}).
		Expect(t, http.StatusOK)

	other := decodeUser(t, f.env.As(f.outsider).Get("/api/users/"+f.member.String()).Expect(t, http.StatusOK))
	if other.FullProfile || other.Email != nil || other.Roles != nil || other.Attributes != nil {
		t.Fatalf("member sees someone else's full profile: %+v", other)
	}
	self := decodeUser(t, f.env.As(f.member).Get("/api/users/"+f.member.String()))
	if !self.FullProfile || self.Email == nil {
		t.Fatalf("self profile = %+v", self)
	}
	byLead := decodeUser(t, f.env.As(f.lead).Get("/api/users/"+f.member.String()))
	if !byLead.FullProfile || string(byLead.Attributes[salida.ID]) != `"2026-12-31"` {
		t.Fatalf("lead view = %+v", byLead)
	}

	admin.Post("/api/attributes/"+salida.ID.String()+"/retire", nil).Expect(t, http.StatusOK)
	byLead = decodeUser(t, f.env.As(f.lead).Get("/api/users/"+f.member.String()))
	if len(byLead.Attributes) != 0 || len(byLead.RetiredAttributes) != 0 {
		t.Fatalf("lead sees retired attribute: %+v", byLead)
	}
	byAdmin := decodeUser(t, admin.Get("/api/users/"+f.member.String()))
	if string(byAdmin.RetiredAttributes[salida.ID]) != `"2026-12-31"` {
		t.Fatalf("admin view = %+v", byAdmin)
	}
	// Retired values are read-only (RF-104).
	admin.Do(http.MethodPatch, "/api/users/"+f.member.String(), map[string]any{"attributes": map[string]any{salida.ID.String(): "2027-01-01"}}).
		Expect(t, http.StatusConflict, "attribute_retired")
	admin.Post("/api/attributes/"+salida.ID.String()+"/restore", nil).Expect(t, http.StatusOK)
	admin.Do(http.MethodPatch, "/api/users/"+f.member.String(), map[string]any{"attributes": map[string]any{salida.ID.String(): "2027-01-01"}}).
		Expect(t, http.StatusOK)
}
