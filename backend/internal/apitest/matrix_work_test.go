package apitest

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// Actors of the spec 004 matrix. Projects and objectives of each operation
// belong to team BI, except for "lead:out", whose ones belong to team Otro.
// "participant" is a member of BI Reportes (a subteam of BI); "owner" is a
// member outside BI who owns the project or objective; "stranger" is a member
// of Otro.
var workActors = []string{"admin", "lead:in", "lead:out", "participant", "owner", "stranger", "viewer"}

type workMatrix struct {
	*Env
	actors         map[string]uuid.UUID
	bi, rep, other uuid.UUID
	admin          *Client
	template       uuid.UUID
	act, cerrado   uuid.UUID
	seq            int
}

func (m *workMatrix) teamFor(actor string) uuid.UUID {
	if actor == "lead:out" {
		return m.other
	}
	return m.bi
}

func (m *workMatrix) identifier() string {
	m.seq++
	return "P" + string(rune('A'+m.seq/26%26)) + string(rune('A'+m.seq%26)) + "X"
}

// project creates a project for the actor's case, owned by "owner".
func (m *workMatrix) project(actor string) uuid.UUID {
	var p projectDTO
	m.admin.Post("/api/projects", map[string]any{"identifier": m.identifier(), "name": "P", "team_id": m.teamFor(actor),
		"owner_id": m.actors["owner"], "methodology_id": m.CatalogID("methodologies", "PRINCE2")}).Expect(m.t, http.StatusCreated).JSON(m.t, &p)
	return p.ID
}

func (m *workMatrix) workItem(project uuid.UUID) workItemDTO {
	var wi workItemDTO
	m.admin.Post("/api/projects/"+project.String()+"/work-items", map[string]any{"type_id": m.act, "title": "x"}).
		Expect(m.t, http.StatusCreated).JSON(m.t, &wi)
	return wi
}

func (m *workMatrix) milestone(project uuid.UUID) milestoneDTO {
	var ms milestoneDTO
	m.admin.Post("/api/projects/"+project.String()+"/milestones", map[string]any{"name": "M"}).Expect(m.t, http.StatusCreated).JSON(m.t, &ms)
	return ms
}

func (m *workMatrix) objective(actor string) objectiveDTO {
	var o objectiveDTO
	m.admin.Post("/api/objectives", map[string]any{"title": "O", "scope": "equipo", "scope_team_id": m.teamFor(actor),
		"owner_id": m.actors["owner"], "period_start": "2026-01-01", "period_end": "2026-03-31"}).Expect(m.t, http.StatusCreated).JSON(m.t, &o)
	return o
}

func (m *workMatrix) project1(actor string) projectDTO {
	var p projectDTO
	m.admin.Get("/api/projects/"+m.project(actor).String()).JSON(m.t, &p)
	return p
}

type workOp struct {
	name string
	do   func(m *workMatrix, c *Client, a string) *Response
	want map[string]int // missing actors expect 403
}

func workAll(status int) map[string]int {
	out := map[string]int{}
	for _, a := range workActors {
		out[a] = status
	}
	return out
}

func managers(status int) map[string]int {
	return map[string]int{"admin": status, "lead:in": status, "owner": status}
}

func writers(status int) map[string]int {
	return map[string]int{"admin": status, "lead:in": status, "owner": status, "participant": status}
}

var workOps = []workOp{
	{"ver catálogo", func(m *workMatrix, c *Client, a string) *Response { return c.Get("/api/catalog") }, workAll(200)},
	{"crear tipo", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/work-item-types", map[string]any{"name": "T-" + uuid.NewString(), "color": "#000000"})
	}, map[string]int{"admin": 201}},
	{"cambiar flujo", func(m *workMatrix, c *Client, a string) *Response {
		return c.Do(http.MethodPut, "/api/work-item-types/"+m.CatalogID("work_item_types", "Story").String()+"/transitions", map[string]any{"transitions": []any{}})
	}, map[string]int{"admin": 200}},
	{"listar campos", func(m *workMatrix, c *Client, a string) *Response { return c.Get("/api/custom-fields") }, workAll(200)},
	{"crear campo", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/custom-fields", map[string]any{"entity": "project", "name": "C-" + uuid.NewString(), "type": "text"})
	}, map[string]int{"admin": 201}},
	{"listar proyectos", func(m *workMatrix, c *Client, a string) *Response { return c.Get("/api/projects") }, workAll(200)},
	{"ver proyecto", func(m *workMatrix, c *Client, a string) *Response {
		return c.Get("/api/projects/" + m.project(a).String())
	}, workAll(200)},
	{"crear proyecto", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/projects", map[string]any{"identifier": m.identifier(), "name": "N", "team_id": m.teamFor(a)})
	}, map[string]int{"admin": 201, "lead:in": 201}},
	{"editar proyecto", func(m *workMatrix, c *Client, a string) *Response {
		p := m.project1(a)
		return c.Do(http.MethodPatch, "/api/projects/"+p.ID.String(), map[string]any{"version": p.Version, "name": "E",
			"team_id": p.Team.ID, "owner_id": p.Owner.ID, "health": "en_riesgo"})
	}, managers(200)},
	{"cerrar proyecto", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/projects/"+m.project(a).String()+"/close", map[string]any{"version": 1})
	}, managers(200)},
	{"activar tipos", func(m *workMatrix, c *Client, a string) *Response {
		return c.Do(http.MethodPut, "/api/projects/"+m.project(a).String()+"/types", map[string]any{"ids": []uuid.UUID{m.act}})
	}, managers(200)},
	{"marcar gate", func(m *workMatrix, c *Client, a string) *Response {
		var phases struct {
			Items []struct{ ID uuid.UUID } `json:"items"`
		}
		m.admin.Get("/api/projects/"+m.project(a).String()+"/phases").JSON(m.t, &phases)
		return c.Post("/api/phases/"+phases.Items[0].ID.String()+"/gates/start", map[string]any{"passed": true})
	}, managers(200)},
	{"marcar plantilla", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/projects/"+m.project(a).String()+"/template", nil)
	}, map[string]int{"admin": 200}},
	{"listar plantillas", func(m *workMatrix, c *Client, a string) *Response { return c.Get("/api/templates") },
		map[string]int{"admin": 200, "lead:in": 200, "lead:out": 200}},
	{"proyecto desde plantilla", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/projects/from-template", map[string]any{"template_id": m.template, "identifier": m.identifier(), "name": "N", "team_id": m.teamFor(a)})
	}, map[string]int{"admin": 201, "lead:in": 201}},
	{"crear hito", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/projects/"+m.project(a).String()+"/milestones", map[string]any{"name": "M"})
	}, managers(201)},
	{"ligar work item a hito", func(m *workMatrix, c *Client, a string) *Response {
		p := m.project(a)
		return c.Post("/api/milestones/"+m.milestone(p).ID.String()+"/work-items", map[string]any{"work_item_id": m.workItem(p).ID})
	}, managers(204)},
	{"cumplir hito", func(m *workMatrix, c *Client, a string) *Response {
		ms := m.milestone(m.project(a))
		return c.Post("/api/milestones/"+ms.ID.String()+"/complete", map[string]any{"version": ms.Version})
	}, managers(200)},
	{"ver árbol", func(m *workMatrix, c *Client, a string) *Response {
		return c.Get("/api/projects/" + m.project(a).String() + "/tree")
	}, workAll(200)},
	{"crear work item", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/projects/"+m.project(a).String()+"/work-items", map[string]any{"type_id": m.act, "title": "x"})
	}, writers(201)},
	{"cerrar work item", func(m *workMatrix, c *Client, a string) *Response {
		wi := m.workItem(m.project(a))
		return c.Do(http.MethodPatch, "/api/work-items/"+wi.ID.String(), map[string]any{"version": wi.Version, "type_id": wi.TypeID,
			"status_id": m.cerrado, "priority_id": wi.PriorityID, "title": wi.Title})
	}, writers(200)},
	{"borrar work item ajeno", func(m *workMatrix, c *Client, a string) *Response {
		return c.Do(http.MethodDelete, "/api/work-items/"+m.workItem(m.project(a)).ID.String(), nil)
	}, managers(204)},
	{"crear dependencia", func(m *workMatrix, c *Client, a string) *Response {
		p := m.project(a)
		return c.Post("/api/work-items/"+m.workItem(p).ID.String()+"/dependencies", map[string]any{"to_id": m.workItem(p).ID, "type": "relates"})
	}, writers(201)},
	{"listar objetivos", func(m *workMatrix, c *Client, a string) *Response { return c.Get("/api/objectives") }, workAll(200)},
	{"crear objetivo de organización", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/objectives", map[string]any{"title": "O", "scope": "organizacion", "period_start": "2026-01-01", "period_end": "2026-03-31"})
	}, map[string]int{"admin": 201}},
	{"crear objetivo de equipo", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/objectives", map[string]any{"title": "O", "scope": "equipo", "scope_team_id": m.teamFor(a), "period_start": "2026-01-01", "period_end": "2026-03-31"})
	}, map[string]int{"admin": 201, "lead:in": 201}},
	{"editar objetivo", func(m *workMatrix, c *Client, a string) *Response {
		o := m.objective(a)
		return c.Do(http.MethodPatch, "/api/objectives/"+o.ID.String(), map[string]any{"version": o.Version, "title": "E",
			"owner_id": m.actors["owner"], "period_start": "2026-01-01", "period_end": "2026-03-31"})
	}, managers(200)},
	{"crear resultado clave", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/objectives/"+m.objective(a).ID.String()+"/key-results", map[string]any{"title": "KR", "start_value": 0, "target_value": 1, "calc_mode": "manual"})
	}, managers(201)},
	{"ligar hito a objetivo", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/objectives/"+m.objective(a).ID.String()+"/milestones", map[string]any{"id": m.milestone(m.project(a)).ID})
	}, managers(200)},
	{"consultar eventos", func(m *workMatrix, c *Client, a string) *Response { return c.Get("/api/events") }, map[string]int{"admin": 200}},
	// Spec 005: files of the pages follow the permission of each owner's edits.
	{"subir archivo a work item", func(m *workMatrix, c *Client, a string) *Response {
		return c.Upload("/api/work-items/"+m.workItem(m.project(a)).ID.String()+"/files", "a.png", pngData)
	}, writers(201)},
	{"subir archivo a hito", func(m *workMatrix, c *Client, a string) *Response {
		return c.Upload("/api/milestones/"+m.milestone(m.project(a)).ID.String()+"/files", "a.png", pngData)
	}, managers(201)},
	{"subir archivo a proyecto", func(m *workMatrix, c *Client, a string) *Response {
		return c.Upload("/api/projects/"+m.project(a).String()+"/files", "a.png", pngData)
	}, managers(201)},
	{"guardar página de work item", func(m *workMatrix, c *Client, a string) *Response {
		return c.Do(http.MethodPut, "/api/work-items/"+m.workItem(m.project(a)).ID.String()+"/page", map[string]any{"page": nil, "page_version": 1})
	}, writers(200)},
	{"guardar página de hito", func(m *workMatrix, c *Client, a string) *Response {
		return c.Do(http.MethodPut, "/api/milestones/"+m.milestone(m.project(a)).ID.String()+"/page", map[string]any{"page": nil, "page_version": 1})
	}, managers(200)},
	{"guardar página de proyecto", func(m *workMatrix, c *Client, a string) *Response {
		return c.Do(http.MethodPut, "/api/projects/"+m.project(a).String()+"/page", map[string]any{"page": nil, "page_version": 1})
	}, managers(200)},
	{"buscar menciones", func(m *workMatrix, c *Client, a string) *Response { return c.Get("/api/mentions?q=x") }, workAll(200)},
	{"descargar archivo", func(m *workMatrix, c *Client, a string) *Response {
		var f fileDTO
		m.admin.Upload("/api/projects/"+m.project(a).String()+"/files", "a.png", pngData).Expect(m.t, http.StatusCreated).JSON(m.t, &f)
		return c.Get("/api/files/" + f.ID.String())
	}, workAll(200)},
}

// TestWorkPermissionMatrix checks every operation of spec 004 against every
// role and scope (RF-123 to RF-125, RF-127).
func TestWorkPermissionMatrix(t *testing.T) {
	env := New(t)
	m := &workMatrix{Env: env}
	adminID := env.CreateUser(UserSpec{Name: "Admin", Email: "admin@x.com", Roles: []string{"admin"}})
	m.admin = env.As(adminID)
	lead := env.CreateUser(UserSpec{Name: "Lead", Email: "lead@x.com", Roles: []string{"lead"}})
	m.bi = env.CreateTeam("BI", nil, lead)
	m.rep = env.CreateTeam("BI Reportes", &m.bi)
	m.other = env.CreateTeam("Otro", nil)
	m.actors = map[string]uuid.UUID{
		"admin": adminID, "lead:in": lead, "lead:out": lead,
		"participant": env.CreateUser(UserSpec{Name: "Part", Email: "part@x.com", Roles: []string{"member"}, Teams: []uuid.UUID{m.rep}}),
		"owner":       env.CreateUser(UserSpec{Name: "Owner", Email: "owner@x.com", Roles: []string{"member"}}),
		"stranger":    env.CreateUser(UserSpec{Name: "Str", Email: "str@x.com", Roles: []string{"member"}, Teams: []uuid.UUID{m.other}}),
		"viewer":      env.CreateUser(UserSpec{Name: "View", Email: "view@x.com", Roles: []string{"viewer"}, Teams: []uuid.UUID{m.bi}}),
	}
	m.act = env.CatalogID("work_item_types", "Actividad")
	m.cerrado = env.CatalogID("statuses", "Cerrado")
	m.template = m.project("admin")
	m.admin.Post("/api/projects/"+m.template.String()+"/template", nil).Expect(t, http.StatusOK)

	for _, op := range workOps {
		for _, actor := range workActors {
			want, ok := op.want[actor]
			if !ok {
				want = http.StatusForbidden
			}
			t.Run(op.name+"/"+actor, func(t *testing.T) {
				res := op.do(m, env.As(m.actors[actor]), actor)
				if res.Status != want {
					t.Fatalf("status = %d, want %d; body %s", res.Status, want, res.Body)
				}
				if want == http.StatusForbidden && res.Code() != "forbidden" {
					t.Fatalf("code = %q, want forbidden", res.Code())
				}
			})
		}
	}
}
