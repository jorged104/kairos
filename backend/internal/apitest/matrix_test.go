package apitest

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"openscum/internal/db/sqlc"
)

// Actors of the permission matrix. "lead:in" acts on a member of a subteam
// of the team it leads; "lead:out" on a member of another team.
var matrixActors = []string{"admin", "lead:in", "lead:out", "member", "viewer"}

type matrixEnv struct {
	*Env
	actors map[string]uuid.UUID
	bi     uuid.UUID
	biRep  uuid.UUID
	other  uuid.UUID
	admin  *Client
}

// target creates a fresh member in the actor's scope (lead:in) or outside it.
func (m *matrixEnv) target(actor string) uuid.UUID {
	team := m.other
	if actor == "lead:in" {
		team = m.biRep
	}
	return m.CreateUser(UserSpec{Email: uuid.NewString() + "@x.com", Roles: []string{"member"}, Teams: []uuid.UUID{team}, Password: pw})
}

func (m *matrixEnv) teamFor(actor string) uuid.UUID {
	if actor == "lead:in" {
		return m.bi
	}
	return m.other
}

func (m *matrixEnv) newTeam() string {
	return m.CreateTeam("T-"+uuid.NewString(), nil).String()
}

func (m *matrixEnv) newAttribute() string {
	var a attrDTO
	m.admin.Post("/api/attributes", map[string]any{"name": "A-" + uuid.NewString(), "type": "text"}).JSON(m.t, &a)
	return a.ID.String()
}

type matrixOp struct {
	name string
	do   func(m *matrixEnv, c *Client, actor string) *Response
	// want is the status per actor; missing actors default to 403.
	want map[string]int
}

func all(status int) map[string]int {
	out := map[string]int{}
	for _, a := range matrixActors {
		out[a] = status
	}
	return out
}

func adminOnly(status int) map[string]int { return map[string]int{"admin": status} }

func adminAndLeadIn(status int) map[string]int {
	return map[string]int{"admin": status, "lead:in": status}
}

var matrixOps = []matrixOp{
	{"ver directorio", func(m *matrixEnv, c *Client, a string) *Response { return c.Get("/api/users") }, all(200)},
	{"ver usuario", func(m *matrixEnv, c *Client, a string) *Response { return c.Get("/api/users/" + m.target(a).String()) }, all(200)},
	{"ver perfil propio", func(m *matrixEnv, c *Client, a string) *Response { return c.Get("/api/me") }, all(200)},
	{"crear usuario", func(m *matrixEnv, c *Client, a string) *Response {
		return c.Post("/api/users", map[string]any{"name": "N", "email": uuid.NewString() + "@x.com", "team_id": m.teamFor(a)})
	}, adminAndLeadIn(201)},
	{"editar usuario", func(m *matrixEnv, c *Client, a string) *Response {
		return c.Do(http.MethodPatch, "/api/users/"+m.target(a).String(), map[string]any{"name": "Editado"})
	}, adminAndLeadIn(200)},
	{"desactivar", func(m *matrixEnv, c *Client, a string) *Response {
		return c.Post("/api/users/"+m.target(a).String()+"/deactivate", nil)
	}, adminAndLeadIn(200)},
	{"reactivar", func(m *matrixEnv, c *Client, a string) *Response {
		id := m.target(a)
		m.admin.Post("/api/users/"+id.String()+"/deactivate", nil)
		return c.Post("/api/users/"+id.String()+"/reactivate", nil)
	}, adminAndLeadIn(200)},
	{"dar de baja", func(m *matrixEnv, c *Client, a string) *Response {
		return c.Post("/api/users/"+m.target(a).String()+"/offboard", nil)
	}, adminOnly(200)},
	{"revertir baja", func(m *matrixEnv, c *Client, a string) *Response {
		id := m.target(a)
		m.admin.Post("/api/users/"+id.String()+"/offboard", nil)
		return c.Post("/api/users/"+id.String()+"/restore", nil)
	}, adminOnly(200)},
	{"fijar contraseña de otro", func(m *matrixEnv, c *Client, a string) *Response {
		return c.Do(http.MethodPut, "/api/users/"+m.target(a).String()+"/password", map[string]any{"password": "nueva-contrasena-x"})
	}, adminOnly(204)},
	{"asignar rol viewer", func(m *matrixEnv, c *Client, a string) *Response {
		return c.Post("/api/users/"+m.target(a).String()+"/roles", map[string]any{"role": "viewer"})
	}, adminAndLeadIn(200)},
	{"asignar rol lead", func(m *matrixEnv, c *Client, a string) *Response {
		return c.Post("/api/users/"+m.target(a).String()+"/roles", map[string]any{"role": "lead"})
	}, adminOnly(200)},
	{"quitar rol member", func(m *matrixEnv, c *Client, a string) *Response {
		return c.Do(http.MethodDelete, "/api/users/"+m.target(a).String()+"/roles/member", nil)
	}, adminAndLeadIn(200)},
	{"añadir a equipo", func(m *matrixEnv, c *Client, a string) *Response {
		return c.Post("/api/users/"+m.target(a).String()+"/teams", map[string]any{"team_id": m.teamFor(a)})
	}, adminAndLeadIn(200)},
	{"quitar de equipo", func(m *matrixEnv, c *Client, a string) *Response {
		team := m.biRep
		if a != "lead:in" {
			team = m.other
		}
		return c.Do(http.MethodDelete, "/api/users/"+m.target(a).String()+"/teams/"+team.String(), nil)
	}, adminAndLeadIn(200)},
	{"listar equipos", func(m *matrixEnv, c *Client, a string) *Response { return c.Get("/api/teams") }, all(200)},
	{"crear equipo", func(m *matrixEnv, c *Client, a string) *Response {
		return c.Post("/api/teams", map[string]any{"name": "T-" + uuid.NewString()})
	}, adminOnly(201)},
	{"renombrar equipo", func(m *matrixEnv, c *Client, a string) *Response {
		return c.Do(http.MethodPatch, "/api/teams/"+m.newTeam(), map[string]any{"name": "R-" + uuid.NewString()})
	}, adminOnly(200)},
	{"archivar equipo", func(m *matrixEnv, c *Client, a string) *Response {
		return c.Post("/api/teams/"+m.newTeam()+"/archive", nil)
	}, adminOnly(200)},
	{"nombrar líder", func(m *matrixEnv, c *Client, a string) *Response {
		return c.Post("/api/teams/"+m.newTeam()+"/leaders", map[string]any{"user_id": m.actors["lead:in"]})
	}, adminOnly(204)},
	{"listar atributos", func(m *matrixEnv, c *Client, a string) *Response { return c.Get("/api/attributes") }, all(200)},
	{"definir atributo", func(m *matrixEnv, c *Client, a string) *Response {
		return c.Post("/api/attributes", map[string]any{"name": "A-" + uuid.NewString(), "type": "text"})
	}, adminOnly(201)},
	{"retirar atributo", func(m *matrixEnv, c *Client, a string) *Response {
		return c.Post("/api/attributes/"+m.newAttribute()+"/retire", nil)
	}, adminOnly(200)},
	{"listar homologaciones", func(m *matrixEnv, c *Client, a string) *Response { return c.Get("/api/idp-mappings") }, adminOnly(200)},
	{"crear homologación", func(m *matrixEnv, c *Client, a string) *Response {
		return c.Post("/api/idp-mappings", map[string]any{"issuer": "https://idp", "claim": "roles", "external_value": uuid.NewString(), "target_type": "role", "target": "member"})
	}, adminOnly(201)},
	{"consultar auditoría", func(m *matrixEnv, c *Client, a string) *Response { return c.Get("/api/audit") }, adminOnly(200)},
	{"cambiar contraseña propia", func(m *matrixEnv, c *Client, a string) *Response {
		res := c.Do(http.MethodPut, "/api/me/password", map[string]any{"current_password": pw, "new_password": "otra-contrasena-1"})
		// Put it back so the next operation still knows it.
		c.Do(http.MethodPut, "/api/me/password", map[string]any{"current_password": "otra-contrasena-1", "new_password": pw})
		return res
	}, all(204)},
}

// TestPermissionMatrix checks every protected operation against every role (RF-62).
func TestPermissionMatrix(t *testing.T) {
	env := New(t)
	m := &matrixEnv{Env: env, actors: map[string]uuid.UUID{}}
	adminID := env.CreateUser(UserSpec{Email: "admin@x.com", Roles: []string{"admin"}, Password: pw})
	m.admin = env.As(adminID)
	lead := env.CreateUser(UserSpec{Email: "lead@x.com", Roles: []string{"lead"}, Password: pw})
	m.bi = env.CreateTeam("BI", nil, lead)
	m.biRep = env.CreateTeam("BI Reportes", &m.bi)
	m.other = env.CreateTeam("Otro", nil)
	m.actors = map[string]uuid.UUID{
		"admin": adminID, "lead:in": lead, "lead:out": lead,
		"member": env.CreateUser(UserSpec{Email: "member@x.com", Roles: []string{"member"}, Password: pw}),
		"viewer": env.CreateUser(UserSpec{Email: "viewer@x.com", Roles: []string{"viewer"}, Password: pw}),
	}
	// A second admin so that no operation trips the last-admin rule.
	if err := env.Store.AddManualRole(context.Background(), sqlc.AddManualRoleParams{
		UserID: env.CreateUser(UserSpec{Email: "admin2@x.com"}), Role: "admin",
	}); err != nil {
		t.Fatal(err)
	}

	for _, op := range matrixOps {
		for _, actor := range matrixActors {
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
