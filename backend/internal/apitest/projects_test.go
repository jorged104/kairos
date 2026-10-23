package apitest

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type projectDTO struct {
	ID         uuid.UUID `json:"id"`
	Identifier string    `json:"identifier"`
	Name       string    `json:"name"`
	Lifecycle  string    `json:"lifecycle"`
	Health     string    `json:"health"`
	Version    int32     `json:"version"`
	Depth      int       `json:"depth"`
	Team       struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	} `json:"team"`
	Owner struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	} `json:"owner"`
	Methodology *struct {
		Name string `json:"name"`
	} `json:"methodology"`
	Parent *struct {
		Identifier string `json:"identifier"`
	} `json:"parent"`
	StartDate *string     `json:"start_date"`
	TypeIDs   []uuid.UUID `json:"work_item_type_ids"`
	Can       struct {
		Manage         bool `json:"manage"`
		WriteWorkItems bool `json:"write_work_items"`
	} `json:"can"`
}

// workSetup is the usual cast of the spec 004 tests: team BI with subteam BI
// Reportes led by lead, a member of BI Reportes and an outsider team.
type workSetup struct {
	env                       *Env
	bi, rep, other            uuid.UUID
	adminID, leadID, memberID uuid.UUID
	admin, lead, member       *Client
	viewer, outsider          *Client
	outsiderID                uuid.UUID
}

func newWorkSetup(t *testing.T) *workSetup {
	t.Helper()
	env := New(t)
	s := &workSetup{env: env}
	s.leadID = env.CreateUser(UserSpec{Name: "Laura", Email: "lead@x.com", Roles: []string{"lead"}})
	s.bi = env.CreateTeam("BI", nil, s.leadID)
	s.rep = env.CreateTeam("BI Reportes", &s.bi)
	s.other = env.CreateTeam("Otro", nil)
	s.adminID = env.CreateUser(UserSpec{Name: "Admin", Email: "admin@x.com", Roles: []string{"admin"}})
	s.memberID = env.CreateUser(UserSpec{Name: "Mario", Email: "member@x.com", Roles: []string{"member"}, Teams: []uuid.UUID{s.rep}})
	s.outsiderID = env.CreateUser(UserSpec{Name: "Otto", Email: "out@x.com", Roles: []string{"member"}, Teams: []uuid.UUID{s.other}})
	s.admin, s.lead, s.member = env.As(s.adminID), env.As(s.leadID), env.As(s.memberID)
	s.outsider = env.As(s.outsiderID)
	s.viewer = env.As(env.CreateUser(UserSpec{Email: "viewer@x.com", Roles: []string{"viewer"}, Teams: []uuid.UUID{s.bi}}))
	return s
}

func (s *workSetup) project(t *testing.T, c *Client, body map[string]any) projectDTO {
	t.Helper()
	if _, ok := body["team_id"]; !ok {
		body["team_id"] = s.bi
	}
	var p projectDTO
	c.Post("/api/projects", body).Expect(t, http.StatusCreated).JSON(t, &p)
	return p
}

func (s *workSetup) patchProject(c *Client, p projectDTO, changes map[string]any) *Response {
	body := map[string]any{
		"version": p.Version, "name": p.Name, "team_id": p.Team.ID, "owner_id": p.Owner.ID,
		"health": p.Health, "start_date": p.StartDate,
	}
	if p.Parent != nil {
		body["parent_id"] = nil
	}
	for k, v := range changes {
		body[k] = v
	}
	return c.Do(http.MethodPatch, "/api/projects/"+p.ID.String(), body)
}

func TestCreateProject(t *testing.T) {
	s := newWorkSetup(t)
	prince := s.env.CatalogID("methodologies", "PRINCE2")

	p := s.project(t, s.lead, map[string]any{
		"identifier": " kai ", "name": "Kairos", "methodology_id": prince, "start_date": "2026-01-01", "end_date": "2026-12-31",
		"description": map[string]any{"type": "doc", "content": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "Hola"}}}}},
	})
	if p.Identifier != "KAI" || p.Owner.ID != s.leadID || p.Team.Name != "BI" || p.Methodology == nil || p.Methodology.Name != "PRINCE2" ||
		p.Lifecycle != "activo" || p.Health != "en_curso" || len(p.TypeIDs) != 6 || !p.Can.Manage {
		t.Fatalf("project = %+v", p)
	}
	var phases struct {
		Items []struct {
			Name         string `json:"name"`
			HasStartGate bool   `json:"has_start_gate"`
		} `json:"items"`
	}
	s.member.Get("/api/projects/"+p.ID.String()+"/phases").Expect(t, http.StatusOK).JSON(t, &phases)
	if len(phases.Items) != 4 || phases.Items[0].Name != "Puesta en marcha" {
		t.Fatalf("phases = %+v (RF-40)", phases.Items)
	}

	// Identifier rules (RF-19, RF-20).
	for _, bad := range []string{"K", "1AB", "AB-C", "ABCDEFGHIJK", "ÁBC"} {
		s.lead.Post("/api/projects", map[string]any{"identifier": bad, "name": "X", "team_id": s.bi}).
			Expect(t, http.StatusBadRequest)
	}
	s.lead.Post("/api/projects", map[string]any{"identifier": "KAI", "name": "Otro", "team_id": s.bi}).Expect(t, http.StatusConflict, "identifier_conflict")
	s.lead.Post("/api/projects", map[string]any{"identifier": "DAT", "name": "X", "team_id": s.bi, "start_date": "2026-02-01", "end_date": "2026-01-01"}).
		Expect(t, http.StatusBadRequest, "invalid_date_range")

	// Owner must be active and not only viewer (RF-25).
	viewerID := s.env.CreateUser(UserSpec{Email: "v2@x.com", Roles: []string{"viewer"}})
	inactive := s.env.CreateUser(UserSpec{Email: "in@x.com", Roles: []string{"member"}, Status: "inactive"})
	for _, owner := range []uuid.UUID{viewerID, inactive, uuid.New()} {
		s.lead.Post("/api/projects", map[string]any{"identifier": "OWN", "name": "X", "team_id": s.bi, "owner_id": owner}).
			Expect(t, http.StatusBadRequest, "invalid_owner")
	}

	// Scope: a lead only creates in its teams; members and viewers never.
	s.lead.Post("/api/projects", map[string]any{"identifier": "OUT", "name": "X", "team_id": s.other}).Expect(t, http.StatusForbidden, "forbidden")
	s.member.Post("/api/projects", map[string]any{"identifier": "MEM", "name": "X", "team_id": s.rep}).Expect(t, http.StatusForbidden, "forbidden")
	s.project(t, s.lead, map[string]any{"identifier": "REP", "name": "Reportes", "team_id": s.rep})

	if got := s.env.AuditActions(p.ID.String()); len(got) != 1 || got[0] != "project.created" {
		t.Fatalf("audit = %v", got)
	}
}

func TestUpdateProjectAndVersion(t *testing.T) {
	s := newWorkSetup(t)
	p := s.project(t, s.lead, map[string]any{"identifier": "KAI", "name": "Kairos"})

	var updated projectDTO
	s.patchProject(s.lead, p, map[string]any{"name": "Kairos 2", "health": "en_riesgo", "health_explanation": "Retraso"}).
		Expect(t, http.StatusOK).JSON(t, &updated)
	if updated.Name != "Kairos 2" || updated.Health != "en_riesgo" || updated.Version != p.Version+1 {
		t.Fatalf("updated = %+v", updated)
	}
	// Same version again: conflict, nothing lost (RF-74).
	s.patchProject(s.lead, p, map[string]any{"name": "Pisado"}).Expect(t, http.StatusConflict, "version_conflict")
	s.patchProject(s.lead, updated, map[string]any{"identifier": "OTRO"}).Expect(t, http.StatusBadRequest, "identifier_immutable")
	s.patchProject(s.lead, updated, map[string]any{"team_id": s.other}).Expect(t, http.StatusForbidden, "forbidden")

	// The owner (a member) manages the project; a participant does not.
	var withOwner projectDTO
	s.patchProject(s.lead, updated, map[string]any{"owner_id": s.memberID}).Expect(t, http.StatusOK).JSON(t, &withOwner)
	var afterMember projectDTO
	s.patchProject(s.member, withOwner, map[string]any{"name": "Por Mario"}).Expect(t, http.StatusOK).JSON(t, &afterMember)
	s.patchProject(s.outsider, afterMember, map[string]any{"name": "X"}).Expect(t, http.StatusForbidden, "forbidden")
	s.patchProject(s.viewer, afterMember, map[string]any{"name": "X"}).Expect(t, http.StatusForbidden, "forbidden")
}

func TestProjectHierarchy(t *testing.T) {
	s := newWorkSetup(t)
	root := s.project(t, s.lead, map[string]any{"identifier": "P0", "name": "Portafolio"})
	parent := root
	chain := []projectDTO{root}
	for i := 1; i < 5; i++ {
		parent = s.project(t, s.lead, map[string]any{"identifier": fmt.Sprintf("P%d", i), "name": "N", "parent_id": parent.ID})
		chain = append(chain, parent)
	}
	if parent.Depth != 4 || parent.Parent == nil || parent.Parent.Identifier != "P3" {
		t.Fatalf("deepest = %+v", parent)
	}
	s.lead.Post("/api/projects", map[string]any{"identifier": "P5", "name": "N", "team_id": s.bi, "parent_id": parent.ID}).
		Expect(t, http.StatusConflict, "project_too_deep")

	// Cycle: the root under its own descendant (RF-27).
	var fresh projectDTO
	s.lead.Get("/api/projects/"+root.ID.String()).JSON(t, &fresh)
	s.patchProject(s.lead, fresh, map[string]any{"parent_id": chain[2].ID}).Expect(t, http.StatusConflict, "project_cycle")
	s.patchProject(s.lead, fresh, map[string]any{"parent_id": root.ID}).Expect(t, http.StatusConflict, "project_cycle")

	// Moving a subtree keeps depths coherent: P3 (with P4) under P0.
	var p3 projectDTO
	s.lead.Get("/api/projects/"+chain[3].ID.String()).JSON(t, &p3)
	s.patchProject(s.lead, p3, map[string]any{"parent_id": root.ID}).Expect(t, http.StatusOK)
	var p4 projectDTO
	s.lead.Get("/api/projects/"+chain[4].ID.String()).JSON(t, &p4)
	if p4.Depth != 2 {
		t.Fatalf("depth after move = %d, want 2", p4.Depth)
	}

	var sub struct {
		Items []struct{ Identifier string } `json:"items"`
	}
	s.viewer.Get("/api/projects/"+root.ID.String()+"/subtree").Expect(t, http.StatusOK).JSON(t, &sub)
	if len(sub.Items) != 4 {
		t.Fatalf("subtree = %+v (RF-28)", sub.Items)
	}
	// Hanging a project under one the caller does not manage is forbidden.
	foreign := s.project(t, s.admin, map[string]any{"identifier": "EXT", "name": "Ajeno", "team_id": s.other})
	s.lead.Post("/api/projects", map[string]any{"identifier": "HIJ", "name": "N", "team_id": s.bi, "parent_id": foreign.ID}).
		Expect(t, http.StatusForbidden, "forbidden")
}

func TestProjectLifecycle(t *testing.T) {
	s := newWorkSetup(t)
	p := s.project(t, s.lead, map[string]any{"identifier": "KAI", "name": "Kairos"})
	path := "/api/projects/" + p.ID.String()

	var closed projectDTO
	s.lead.Post(path+"/close", map[string]any{"version": p.Version}).Expect(t, http.StatusOK).JSON(t, &closed)
	s.patchProject(s.lead, closed, map[string]any{"name": "X"}).Expect(t, http.StatusConflict, "project_closed")
	s.lead.Post(path+"/close", map[string]any{"version": closed.Version}).Expect(t, http.StatusConflict, "invalid_lifecycle")
	s.lead.Do(http.MethodPut, path+"/types", map[string]any{"ids": []uuid.UUID{}}).Expect(t, http.StatusConflict, "project_closed")

	var archived projectDTO
	s.lead.Post(path+"/archive", map[string]any{"version": closed.Version}).Expect(t, http.StatusOK).JSON(t, &archived)
	s.patchProject(s.lead, archived, map[string]any{"name": "X"}).Expect(t, http.StatusConflict, "project_archived")

	var list struct {
		Items []projectDTO `json:"items"`
		Total int          `json:"total"`
	}
	s.member.Get("/api/projects").Expect(t, http.StatusOK).JSON(t, &list)
	if list.Total != 0 {
		t.Fatalf("archived project listed by default: %+v (RF-30)", list.Items)
	}
	s.member.Get("/api/projects?lifecycle=archivado").Expect(t, http.StatusOK).JSON(t, &list)
	if list.Total != 1 {
		t.Fatalf("archived filter = %d", list.Total)
	}
	s.lead.Post(path+"/unarchive", map[string]any{"version": archived.Version}).Expect(t, http.StatusOK)
	s.lead.Do(http.MethodDelete, path, nil).Expect(t, http.StatusMethodNotAllowed) // RF-31
}

func TestProjectTypes(t *testing.T) {
	s := newWorkSetup(t)
	p := s.project(t, s.lead, map[string]any{"identifier": "KAI", "name": "Kairos", "work_item_type_ids": []uuid.UUID{}})
	if len(p.TypeIDs) != 0 {
		t.Fatalf("types = %v, want none", p.TypeIDs)
	}
	act := s.env.CatalogID("work_item_types", "Actividad")
	bug := s.env.CatalogID("work_item_types", "Bug")
	path := "/api/projects/" + p.ID.String() + "/types"
	var got projectDTO
	s.lead.Do(http.MethodPut, path, map[string]any{"ids": []uuid.UUID{act, bug}}).Expect(t, http.StatusOK).JSON(t, &got)
	if len(got.TypeIDs) != 2 {
		t.Fatalf("types = %v", got.TypeIDs)
	}
	nuevo := s.env.CatalogID("statuses", "Nuevo")
	s.env.SeedWorkItem(p.ID, nuevo, s.leadID) // an Actividad
	s.lead.Do(http.MethodPut, path, map[string]any{"ids": []uuid.UUID{bug}}).Expect(t, http.StatusConflict, "type_in_use")
	s.lead.Do(http.MethodPut, path, map[string]any{"ids": []uuid.UUID{act}}).Expect(t, http.StatusOK)
	s.member.Do(http.MethodPut, path, map[string]any{"ids": []uuid.UUID{act, bug}}).Expect(t, http.StatusForbidden, "forbidden")
}

func TestProjectListFilters(t *testing.T) {
	s := newWorkSetup(t)
	s.project(t, s.lead, map[string]any{"identifier": "KAI", "name": "Kairós"})
	s.project(t, s.lead, map[string]any{"identifier": "REP", "name": "Reportes", "team_id": s.rep, "health": "en_riesgo"})
	s.project(t, s.admin, map[string]any{"identifier": "EXT", "name": "Externo", "team_id": s.other, "owner_id": s.outsiderID})

	count := func(query string) int {
		t.Helper()
		var list struct {
			Total int `json:"total"`
		}
		s.viewer.Get("/api/projects"+query).Expect(t, http.StatusOK).JSON(t, &list)
		return list.Total
	}
	cases := map[string]int{
		"":                                   3,
		"?q=kairos":                          1, // accents ignored
		"?q=rep":                             1,
		"?team_id=" + s.bi.String():          2, // subteams included
		"?health=en_riesgo":                  1,
		"?owner_id=" + s.outsiderID.String(): 1,
		"?page_size=1&page=3":                3,
	}
	for q, want := range cases {
		if got := count(q); got != want {
			t.Errorf("%q: total = %d, want %d", q, got, want)
		}
	}
	for _, bad := range []string{"?team_id=x", "?lifecycle=raro", "?health=bien", "?page_size=101", "?owner_id=1"} {
		s.viewer.Get("/api/projects"+bad).Expect(t, http.StatusBadRequest, "invalid_filter")
	}
}

func TestPhasesAndGates(t *testing.T) {
	s := newWorkSetup(t)
	p := s.project(t, s.lead, map[string]any{"identifier": "KAI", "name": "Kairos",
		"methodology_id": s.env.CatalogID("methodologies", "PRINCE2"), "start_date": "2026-01-01", "end_date": "2026-06-30"})
	var phases struct {
		Items []struct {
			ID uuid.UUID `json:"id"`
		} `json:"items"`
	}
	s.lead.Get("/api/projects/"+p.ID.String()+"/phases").JSON(t, &phases)
	ph := "/api/phases/" + phases.Items[0].ID.String()

	s.lead.Do(http.MethodPatch, ph, map[string]any{"start_date": "2026-01-01", "finish_date": "2026-02-01"}).Expect(t, http.StatusOK)
	s.lead.Do(http.MethodPatch, ph, map[string]any{"start_date": "2025-12-01"}).Expect(t, http.StatusBadRequest, "invalid_date_range")
	s.lead.Do(http.MethodPatch, ph, map[string]any{"start_date": "2026-03-01", "finish_date": "2026-02-01"}).Expect(t, http.StatusBadRequest, "invalid_date_range")

	var gate struct {
		StartGatePassedAt *string    `json:"start_gate_passed_at"`
		StartGatePassedBy *uuid.UUID `json:"start_gate_passed_by"`
	}
	s.lead.Post(ph+"/gates/start", map[string]any{"passed": true}).Expect(t, http.StatusOK).JSON(t, &gate)
	if gate.StartGatePassedAt == nil || gate.StartGatePassedBy == nil || *gate.StartGatePassedBy != s.leadID {
		t.Fatalf("gate = %+v (RF-42)", gate)
	}
	s.lead.Post(ph+"/gates/middle", map[string]any{"passed": true}).Expect(t, http.StatusNotFound)
	s.member.Post(ph+"/gates/finish", map[string]any{"passed": true}).Expect(t, http.StatusForbidden, "forbidden")
}

// Empty lists must reach the client as [] (the UI calls .includes and .length on them).
func TestEmptyListsAreArrays(t *testing.T) {
	s := newWorkSetup(t)
	p := s.project(t, s.lead, map[string]any{"identifier": "EMP", "name": "Vacío", "work_item_type_ids": []uuid.UUID{}})
	res := s.lead.Get("/api/projects/" + p.ID.String())
	for _, field := range []string{`"work_item_type_ids":[]`, `"custom_field_ids":[]`} {
		if !strings.Contains(string(res.Body), field) {
			t.Fatalf("project lacks %s: %s", field, res.Body)
		}
	}
	act := s.env.CatalogID("work_item_types", "Actividad")
	s.lead.Do(http.MethodPut, "/api/projects/"+p.ID.String()+"/types", map[string]any{"ids": []uuid.UUID{act}}).Expect(t, http.StatusOK)
	// A type whose workflow allows nothing from the current status.
	nuevo, prog := s.env.CatalogID("statuses", "Nuevo"), s.env.CatalogID("statuses", "En progreso")
	s.admin.Do(http.MethodPut, "/api/work-item-types/"+act.String()+"/transitions", map[string]any{"transitions": []map[string]any{
		{"from_status_id": prog, "to_status_id": nuevo},
	}}).Expect(t, http.StatusOK)
	wi := s.item(t, s.member, p.ID, map[string]any{})
	res = s.member.Get("/api/work-items/" + wi.ID.String())
	for _, field := range []string{`"allowed_status_ids":[]`, `"dependencies":[]`} {
		if !strings.Contains(string(res.Body), field) {
			t.Fatalf("work item lacks %s: %s", field, res.Body)
		}
	}
}
