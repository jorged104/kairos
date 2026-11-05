package mcp

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"openscum/internal/richtext"
)

// The meetings of spec 018 through the AI clients (RF-45 to RF-54): to read,
// create and complete them; never to delete them or manage templates.

var pMeeting = prop("string", "Id de la reunión.")

const msgTimeZone = "La fecha y hora deben indicar la zona horaria, por ejemplo 2026-11-10T10:00:00-06:00 o 2026-11-10T16:00:00Z."

// startsAt checks that a date and time says its time zone (RF-52).
func startsAt(v string) (string, error) {
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return "", argError(msgTimeZone)
	}
	return t.Format(time.RFC3339), nil
}

func (c *call) meetingURL(v any) {
	switch x := v.(type) {
	case map[string]any:
		if id := str(x["id"]); id != "" && x["starts_at"] != nil {
			x["url"] = c.base + "/meetings/" + id
		}
		c.meetingURL(x["items"])
	case []any:
		for _, it := range x {
			c.meetingURL(it)
		}
	}
}

func meetingTools() []tool {
	return []tool{
		{
			name: "list_meetings", title: "Reuniones del proyecto", readOnly: true,
			description: "Las reuniones de un proyecto o programa: las próximas (de la más cercana a la más lejana) o las pasadas (de la más reciente a la más antigua).",
			params: map[string]any{
				"project": pProject,
				"when":    enum("Próximas o pasadas. Por defecto, próximas.", "upcoming", "past"),
				"page":    pPage,
			},
			required: []string{"project"},
			run: func(c *call, a args) (any, error) {
				pid, err := c.projectID(a.str("project"))
				if err != nil {
					return nil, err
				}
				when := a.str("when")
				if when == "" {
					when = "upcoming"
				}
				out, err := c.do(http.MethodGet, "/projects/"+pid+"/meetings", url.Values{"when": {when}, "page": {a.page()}, "page_size": {strconv.Itoa(PageSize)}}, nil)
				c.meetingURL(out)
				return out, err
			},
		},
		{
			name: "get_meeting", title: "Ver reunión", readOnly: true,
			description: "Una reunión con sus notas en Markdown, las tareas que salieron de ella y sus comentarios.",
			params:      map[string]any{"meeting": pMeeting}, required: []string{"meeting"},
			run: func(c *call, a args) (any, error) {
				mid, err := id(a.str("meeting"), "meeting")
				if err != nil {
					return nil, err
				}
				m, err := c.get("/meetings/"+mid, nil)
				if err != nil {
					return nil, err
				}
				if m["comments"], err = c.do(http.MethodGet, "/meetings/"+mid+"/comments", nil, nil); err != nil {
					return nil, err
				}
				c.meetingURL(m)
				return m, nil
			},
		},
		{
			name: "list_meeting_templates", title: "Plantillas de reunión", readOnly: true,
			description: "Las plantillas que puede usar una reunión nueva del proyecto: las globales y las del proyecto.",
			params:      map[string]any{"project": pProject}, required: []string{"project"},
			run: func(c *call, a args) (any, error) {
				pid, err := c.projectID(a.str("project"))
				if err != nil {
					return nil, err
				}
				return c.do(http.MethodGet, "/meeting-templates", url.Values{"project_id": {pid}}, nil)
			},
		},
		{
			name: "create_meeting", title: "Crear reunión",
			description: "Crea una reunión en un proyecto o programa. Con plantilla, sus notas van tras la página de la plantilla, y el título y el lugar vacíos toman los de la plantilla.",
			params: map[string]any{
				"project":     pProject,
				"title":       prop("string", "Título. Obligatorio si no hay plantilla."),
				"starts_at":   prop("string", "Fecha y hora con zona horaria (RFC 3339), por ejemplo 2026-11-10T10:00:00-06:00."),
				"location":    prop("string", "Lugar o enlace de la videollamada."),
				"template_id": prop("string", "Id de la plantilla (ver list_meeting_templates)."),
				"notes":       pMarkdown,
			},
			required: []string{"project", "starts_at"},
			run: func(c *call, a args) (any, error) {
				pid, err := c.projectID(a.str("project"))
				if err != nil {
					return nil, err
				}
				at, err := startsAt(a.str("starts_at"))
				if err != nil {
					return nil, err
				}
				body := map[string]any{"title": a.str("title"), "starts_at": at}
				if a.has("location") {
					body["location"] = a.str("location")
				}
				if t := a.str("template_id"); t != "" {
					body["template_id"] = t
				}
				if n := a.str("notes"); n != "" {
					body["description"] = richtext.FromMarkdown(n)
				}
				out, err := c.do(http.MethodPost, "/projects/"+pid+"/meetings", nil, body)
				c.meetingURL(out)
				return out, err
			},
		},
		{
			name: "update_meeting", title: "Editar reunión",
			description: "Cambia el título, la fecha y hora o el lugar de una reunión. Para las notas usa update_page con kind=meeting.",
			params: map[string]any{
				"meeting":   pMeeting,
				"title":     prop("string", "Título."),
				"starts_at": prop("string", "Fecha y hora con zona horaria (RFC 3339)."),
				"location":  prop("string", "Lugar o enlace."),
			},
			required: []string{"meeting"},
			run: func(c *call, a args) (any, error) {
				mid, err := id(a.str("meeting"), "meeting")
				if err != nil {
					return nil, err
				}
				m, err := c.get("/meetings/"+mid, nil)
				if err != nil {
					return nil, err
				}
				body := map[string]any{"version": m["version"], "title": m["title"], "starts_at": m["starts_at"], "location": m["location"]}
				if a.has("starts_at") {
					if body["starts_at"], err = startsAt(a.str("starts_at")); err != nil {
						return nil, err
					}
				}
				a.set(body, map[string]string{"title": "title", "location": "location"})
				out, err := c.do(http.MethodPatch, "/meetings/"+mid, nil, body)
				c.meetingURL(out)
				return out, err
			},
		},
		{
			name: "add_meeting_comment", title: "Comentar en una reunión",
			description: "Añade un comentario a una reunión, o responde a un hilo.",
			params: map[string]any{
				"meeting":  pMeeting,
				"text":     pMarkdown,
				"reply_to": prop("string", "Id del comentario al que se responde."),
			},
			required: []string{"meeting", "text"},
			run: func(c *call, a args) (any, error) {
				mid, err := id(a.str("meeting"), "meeting")
				if err != nil {
					return nil, err
				}
				text, err := a.need("text")
				if err != nil {
					return nil, err
				}
				body := map[string]any{"body": richtext.FromMarkdown(text)}
				if r := a.str("reply_to"); r != "" {
					body["thread_id"] = r
				}
				return c.do(http.MethodPost, "/meetings/"+mid+"/comments", nil, body)
			},
		},
		{
			name: "create_meeting_task", title: "Crear tarea desde una reunión",
			description: "Crea en el proyecto de la reunión una tarea enlazada a ella y la menciona al final de sus notas. Consulta antes project_catalog para el tipo.",
			params: with(workItemFields(), map[string]any{
				"meeting":     pMeeting,
				"description": pMarkdown,
			}),
			required: []string{"meeting", "type_id", "title"},
			run: func(c *call, a args) (any, error) {
				mid, err := id(a.str("meeting"), "meeting")
				if err != nil {
					return nil, err
				}
				m, err := c.get("/meetings/"+mid, nil)
				if err != nil {
					return nil, err
				}
				project, _ := m["project"].(map[string]any)
				body := map[string]any{"meeting_id": mid}
				a.set(body, workItemArgs)
				if a.has("description") {
					body["description"] = richtext.FromMarkdown(a.str("description"))
				}
				created, err := c.do(http.MethodPost, "/projects/"+str(project["id"])+"/work-items", nil, body)
				if err != nil {
					return nil, err
				}
				task, _ := created.(map[string]any)
				// The new work item, mentioned at the end of the notes (RF-25).
				mention, _ := json.Marshal(map[string]any{"type": "doc", "content": []any{map[string]any{
					"type": "paragraph", "content": []any{
						map[string]any{"type": "text", "text": "Tarea: "},
						map[string]any{"type": "mention", "attrs": map[string]any{"kind": "work_item", "id": task["id"]}},
					},
				}}})
				page := appendDoc(m["description"], mention)
				if _, err := c.do(http.MethodPut, "/meetings/"+mid+"/page", nil, map[string]any{"page": json.RawMessage(page), "page_version": m["page_version"]}); err != nil {
					return nil, err
				}
				return task, nil
			},
		},
	}
}
