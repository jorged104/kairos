package apitest

import (
	"encoding/json"
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

func (m *workMatrix) seqNext() int {
	m.seq++
	return m.seq
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

// commenters: every role but viewer, in any project they can read (spec 006, RF-23).
func commenters(status int) map[string]int {
	out := workAll(status)
	delete(out, "viewer")
	return out
}

// comment adds a thread by c to a new work item of project.
func (m *workMatrix) comment(project uuid.UUID, c *Client) (wi, root uuid.UUID) {
	var res commentsResult
	wi = m.workItem(project).ID
	c.Post("/api/work-items/"+wi.String()+"/comments", map[string]any{"body": json.RawMessage(paragraph("hola"))}).
		Expect(m.t, http.StatusCreated).JSON(m.t, &res)
	return wi, res.Comments.Threads[len(res.Comments.Threads)-1].ID
}

var workOps = []workOp{
	{"ver catálogo", func(m *workMatrix, c *Client, a string) *Response { return c.Get("/api/catalog") }, workAll(200)},
	{"crear tipo", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/work-item-types", map[string]any{"name": "T-" + uuid.NewString(), "color": "#000000"})
	}, map[string]int{"admin": 201}},
	{"cambiar flujo", func(m *workMatrix, c *Client, a string) *Response {
		return c.Do(http.MethodPut, "/api/work-item-types/"+m.CatalogID("work_item_types", "Story").String()+"/transitions", map[string]any{"transitions": []any{}})
	}, map[string]int{"admin": 200}},
	{"crear etiqueta", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/labels", map[string]any{"name": "E-" + uuid.NewString(), "color": "#000000"})
	}, map[string]int{"admin": 201}},
	{"listar campos", func(m *workMatrix, c *Client, a string) *Response { return c.Get("/api/custom-fields") }, workAll(200)},
	{"crear campo", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/custom-fields", map[string]any{"entity": "project", "name": "C-" + uuid.NewString(), "type": "text"})
	}, map[string]int{"admin": 201}},
	{"listar proyectos", func(m *workMatrix, c *Client, a string) *Response { return c.Get("/api/projects") }, workAll(200)},
	{"ver proyecto", func(m *workMatrix, c *Client, a string) *Response {
		return c.Get("/api/projects/" + m.project(a).String())
	}, workAll(200)},
	// Spec 009: the plan is seen with the project; the baseline is for whoever
	// manages it and the traffic light for the administrator.
	{"ver plan del proyecto", func(m *workMatrix, c *Client, a string) *Response {
		return c.Get("/api/projects/" + m.project(a).String() + "/schedule")
	}, workAll(200)},
	{"fijar línea base", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/projects/"+m.project(a).String()+"/baseline", map[string]any{})
	}, managers(201)},
	// Spec 010: everyone sees the portfolio; only the administrator marks programs.
	{"ver seguimiento", func(m *workMatrix, c *Client, a string) *Response { return c.Get("/api/tracking") }, workAll(200)}, // spec 011
	{"ver portafolio", func(m *workMatrix, c *Client, a string) *Response { return c.Get("/api/portfolio") }, workAll(200)},
	{"gestionar programa", func(m *workMatrix, c *Client, a string) *Response {
		return c.Do(http.MethodPut, "/api/projects/"+m.project(a).String()+"/portfolio", map[string]any{"version": 1, "is_program": true, "weight": nil})
	}, map[string]int{"admin": 200}},
	// Spec 012: everyone reads the board and keeps personal views; whoever
	// manages a project moves it; leads and the administrator share views.
	{"ver mi panel", func(m *workMatrix, c *Client, a string) *Response { return c.Get("/api/me/dashboard") }, workAll(200)}, // spec 013
	// Spec 014: everyone sees their points and the tables; only the administrator changes them.
	{"ver mis puntos", func(m *workMatrix, c *Client, a string) *Response { return c.Get("/api/me/points") }, workAll(200)},
	{"ver tabla de puntos", func(m *workMatrix, c *Client, a string) *Response { return c.Get("/api/gamification/values") }, workAll(200)},
	{"cambiar tabla de puntos", func(m *workMatrix, c *Client, a string) *Response {
		return c.Do(http.MethodPut, "/api/gamification/values", map[string]any{"no_size": map[string]any{"xp": 5, "coins": 1}})
	}, map[string]int{"admin": 200}},
	{"crear nivel", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/gamification/levels", map[string]any{"name": "N", "min_xp": 10000 + m.seqNext()})
	}, map[string]int{"admin": 201}},
	{"ajustar puntos", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/gamification/users/"+m.actors["owner"].String()+"/adjustments", map[string]any{"xp": 1, "coins": 0, "note": "matriz"})
	}, map[string]int{"admin": 201}},
	// Spec 015: only the administrator of the system sees and changes its settings.
	{"ver configuración del sistema", func(m *workMatrix, c *Client, a string) *Response { return c.Get("/api/system/settings") }, map[string]int{"admin": 200}},
	{"cambiar apariencia", func(m *workMatrix, c *Client, a string) *Response {
		return c.Do(http.MethodPut, "/api/system/settings/appearance", map[string]any{"palette": "neutral", "primary": "default", "radius": "soft"})
	}, map[string]int{"admin": 200}},
	// Spec 016: the switch and everybody's connections are the system administrator's;
	// each person sees their own.
	{"activar acceso por IA", func(m *workMatrix, c *Client, a string) *Response {
		return c.Do(http.MethodPut, "/api/system/settings/mcp", map[string]any{"enabled": true})
	}, map[string]int{"admin": 200}},
	{"ver conexiones de IA de todos", func(m *workMatrix, c *Client, a string) *Response { return c.Get("/api/system/mcp-connections") }, map[string]int{"admin": 200}},
	// Spec 017: anybody who reads the work follows work items and reads their own notifications.
	{"seguir tarea", func(m *workMatrix, c *Client, a string) *Response {
		return c.Do(http.MethodPut, "/api/work-items/"+m.workItem(m.project(a)).ID.String()+"/follow", map[string]any{"following": true})
	}, workAll(200)},
	// Spec 018: meetings as work items, their comments as comments, templates by scope.
	{"crear reunión", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/projects/"+m.project(a).String()+"/meetings", map[string]any{"title": "R", "starts_at": "2030-01-01T10:00:00Z"})
	}, writers(201)},
	{"comentar reunión", func(m *workMatrix, c *Client, a string) *Response {
		var mt meetingDTO
		m.admin.Post("/api/projects/"+m.project(a).String()+"/meetings", map[string]any{"title": "R", "starts_at": "2030-01-01T10:00:00Z"}).
			Expect(m.t, http.StatusCreated).JSON(m.t, &mt)
		return c.Post("/api/meetings/"+mt.ID.String()+"/comments", map[string]any{"body": json.RawMessage(paragraph("hola"))})
	}, commenters(201)},
	{"crear plantilla global de reunión", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/meeting-templates", map[string]any{"name": "T-" + uuid.NewString()})
	}, map[string]int{"admin": 201}},
	{"crear plantilla de reunión del proyecto", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/meeting-templates", map[string]any{"name": "T", "project_id": m.project(a)})
	}, managers(201)},
	// Spec 020: everyone reads the reports of a project; whoever manages it writes
	// and generates them; templates and the AI service are the system administrator's.
	{"ver informes del proyecto", func(m *workMatrix, c *Client, a string) *Response {
		return c.Get("/api/projects/" + m.project(a).String() + "/reports")
	}, workAll(200)},
	{"crear informe", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/projects/"+m.project(a).String()+"/reports", map[string]any{"title": "I", "content": "# I", "origin": "manual"})
	}, managers(201)},
	{"generar informe sin IA configurada", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/projects/"+m.project(a).String()+"/reports/generate", map[string]any{"draft": "# I", "change": "más corto"})
	}, managers(409)},
	{"ver plantillas de informe", func(m *workMatrix, c *Client, a string) *Response { return c.Get("/api/report-templates") }, workAll(200)},
	{"crear plantilla de informe", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/system/report-templates", map[string]any{"name": "T-" + uuid.NewString(), "prompt": "p"})
	}, map[string]int{"admin": 201}},
	{"configurar servicio de IA", func(m *workMatrix, c *Client, a string) *Response {
		return c.Do(http.MethodPut, "/api/system/settings/ai", map[string]any{"enabled": false})
	}, map[string]int{"admin": 200}},
	{"ver mis notificaciones", func(m *workMatrix, c *Client, a string) *Response { return c.Get("/api/notifications") }, workAll(200)},
	{"ver mis conexiones de IA", func(m *workMatrix, c *Client, a string) *Response { return c.Get("/api/me/mcp-connections") }, workAll(200)},
	{"ver puntos de otro", func(m *workMatrix, c *Client, a string) *Response {
		return c.Get("/api/gamification/users/" + m.actors["stranger"].String() + "/points")
	}, map[string]int{"admin": 200}},
	{"ver tablero de proyectos", func(m *workMatrix, c *Client, a string) *Response { return c.Get("/api/project-board") }, workAll(200)},
	{"crear estado de proyecto", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/project-statuses", map[string]any{"name": "E-" + uuid.NewString(), "color": "#000000"})
	}, map[string]int{"admin": 201}},
	{"cambiar estado de proyecto", func(m *workMatrix, c *Client, a string) *Response {
		return c.Do(http.MethodPut, "/api/projects/"+m.project(a).String()+"/status", map[string]any{"version": 1, "status_id": m.CatalogID("project_statuses", "Planificación")})
	}, managers(200)},
	{"guardar vista de proyectos", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/project-views", map[string]any{"name": "V-" + uuid.NewString(), "config": map[string]any{}})
	}, workAll(201)},
	{"compartir vista de proyectos", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/project-views", map[string]any{"name": "V-" + uuid.NewString(), "shared": true, "config": map[string]any{}})
	}, map[string]int{"admin": 201, "lead:in": 201, "lead:out": 201}},
	{"ver semáforo", func(m *workMatrix, c *Client, a string) *Response { return c.Get("/api/schedule-settings") }, workAll(200)},
	{"configurar semáforo", func(m *workMatrix, c *Client, a string) *Response {
		var cur map[string]any
		m.admin.Get("/api/schedule-settings").Expect(m.t, http.StatusOK).JSON(m.t, &cur)
		return c.Do(http.MethodPut, "/api/schedule-settings", cur)
	}, map[string]int{"admin": 200}},
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
	// Team of the project (spec 008, RF-4, RF-33, RF-52): its managers.
	{"ver equipo del proyecto", func(m *workMatrix, c *Client, a string) *Response {
		return c.Get("/api/projects/" + m.project(a).String() + "/members")
	}, managers(200)},
	{"dar papel en el proyecto", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/projects/"+m.project(a).String()+"/members", map[string]any{"user_id": m.actors["stranger"], "role": "member"})
	}, managers(200)},
	{"ver sugerencias de asignación", func(m *workMatrix, c *Client, a string) *Response {
		return c.Get("/api/projects/" + m.project(a).String() + "/assignment-suggestions")
	}, managers(200)},
	{"crear hito", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/projects/"+m.project(a).String()+"/milestones", map[string]any{"name": "M"})
	}, managers(201)},
	{"ligar work item a hito", func(m *workMatrix, c *Client, a string) *Response {
		p := m.project(a)
		return c.Post("/api/milestones/"+m.milestone(p).ID.String()+"/work-items", map[string]any{"work_item_id": m.workItem(p).ID})
	}, writers(204)},
	{"desligar work items de un hito", func(m *workMatrix, c *Client, a string) *Response {
		p := m.project(a)
		return c.Post("/api/milestones/"+m.milestone(p).ID.String()+"/work-items/unlink", map[string]any{"work_item_ids": []uuid.UUID{m.workItem(p).ID}})
	}, writers(200)},
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
	{"leer comentarios", func(m *workMatrix, c *Client, a string) *Response {
		return c.Get("/api/work-items/" + m.workItem(m.project(a)).ID.String() + "/comments")
	}, workAll(200)},
	{"comentar", func(m *workMatrix, c *Client, a string) *Response {
		return c.Post("/api/work-items/"+m.workItem(m.project(a)).ID.String()+"/comments", map[string]any{"body": json.RawMessage(paragraph("hola"))})
	}, commenters(201)},
	{"responder comentario", func(m *workMatrix, c *Client, a string) *Response {
		wi, root := m.comment(m.project(a), m.admin)
		return c.Post("/api/work-items/"+wi.String()+"/comments", map[string]any{"body": json.RawMessage(paragraph("sí")), "thread_id": root})
	}, commenters(201)},
	{"editar comentario ajeno", func(m *workMatrix, c *Client, a string) *Response {
		_, root := m.comment(m.project(a), m.admin)
		return c.Do(http.MethodPatch, "/api/comments/"+root.String(), map[string]any{"body": json.RawMessage(paragraph("otro"))})
	}, map[string]int{"admin": 200}},
	{"borrar comentario de participante", func(m *workMatrix, c *Client, a string) *Response {
		_, root := m.comment(m.project(a), m.As(m.actors["participant"]))
		return c.Do(http.MethodDelete, "/api/comments/"+root.String(), nil)
	}, map[string]int{"admin": 200, "lead:in": 200, "owner": 200, "participant": 200}},
	{"resolver hilo", func(m *workMatrix, c *Client, a string) *Response {
		_, root := m.comment(m.project(a), m.admin)
		return c.Post("/api/comments/"+root.String()+"/resolve", nil)
	}, commenters(200)},
	{"subir archivo a comentario", func(m *workMatrix, c *Client, a string) *Response {
		return c.Upload("/api/work-items/"+m.workItem(m.project(a)).ID.String()+"/comment-files", "a.png", pngData)
	}, commenters(201)},
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
