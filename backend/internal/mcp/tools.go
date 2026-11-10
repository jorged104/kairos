package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"openscum/internal/auth"
	"openscum/internal/authz"
	"openscum/internal/richtext"
)

// PageSize is the most items a list returns at once (RF-39).
const PageSize = 50

// tool is one operation offered to the clients (RF-29 to RF-33). None of
// them deletes, archives, cancels or closes anything (RF-31).
type tool struct {
	name        string
	title       string
	description string
	readOnly    bool
	params      map[string]any // the properties of the input schema
	required    []string
	run         func(c *call, a args) (any, error)
}

func (t tool) definition() map[string]any {
	schema := map[string]any{"type": "object", "properties": t.params, "additionalProperties": false}
	if len(t.required) > 0 {
		schema["required"] = t.required
	}
	return map[string]any{
		"name": t.name, "title": t.title, "description": t.description, "inputSchema": schema,
		"annotations": map[string]any{
			"title": t.title, "readOnlyHint": t.readOnly, "destructiveHint": false,
			"idempotentHint": t.readOnly, "openWorldHint": false,
		},
	}
}

// call runs the tool and wraps what it returns, or its error, as a tool
// result: the errors of the API go to the client, not to the protocol (RF-41).
func (s *Server) call(ctx context.Context, p *authz.Principal, t tool, raw map[string]any) map[string]any {
	c := &call{s: s, ctx: ctx, p: p, base: strings.TrimRight(s.conns.publicURL(), "/")}
	out, err := t.run(c, args(raw))
	if err != nil {
		var text string
		var ae *apiError
		switch {
		case errors.As(err, &ae):
			text = ae.Error()
		case errors.As(err, new(argError)):
			text = err.Error()
		default:
			s.logger.Error("herramienta MCP", "tool", t.name, "error", err)
			text = "Error interno de Kairos."
		}
		return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}, "isError": true}
	}
	b, _ := json.MarshalIndent(c.shape(out, nil), "", "  ")
	return map[string]any{"content": []map[string]any{{"type": "text", "text": string(b)}}, "isError": false}
}

// argError is a wrong argument, explained in Spanish (RF-41).
type argError string

func (e argError) Error() string { return string(e) }

// apiError is the answer of an API route that failed, with the same code
// and message the interface would get (RF-28, RF-40, RF-41).
type apiError struct {
	Status  int
	Code    string
	Message string
	Details map[string]any
}

func (e *apiError) Error() string {
	s := e.Code + ": " + e.Message
	if len(e.Details) > 0 {
		d, _ := json.Marshal(e.Details)
		s += " " + string(d)
	}
	switch e.Code {
	case "version_conflict":
		s += ". Otra persona cambió el elemento: vuelve a leerlo antes de reintentar."
	case "not_found":
		s += ". No existe o no tienes acceso."
	}
	return s
}

type call struct {
	s    *Server
	ctx  context.Context
	p    *authz.Principal
	base string
	max  int // the longest list shape keeps; 0, PageSize
}

func (c *call) limit() int {
	if c.max > 0 {
		return c.max
	}
	return PageSize
}

// do serves one API route inside the process, as the connection's owner.
func (c *call) do(method, path string, query url.Values, body any) (any, error) {
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("serializar petición: %w", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	target := "/api" + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	// A fresh routing context: the one of the MCP request would be reused.
	ctx := context.WithValue(auth.Internal(c.ctx, c.p), chi.RouteCtxKey, nil)
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, fmt.Errorf("crear petición: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c.s.api.ServeHTTP(rec, req)
	if rec.Code >= 300 {
		var e struct {
			Error struct {
				Code    string         `json:"code"`
				Message string         `json:"message"`
				Details map[string]any `json:"details"`
			} `json:"error"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &e)
		if e.Error.Code == "" {
			e.Error.Code, e.Error.Message = "error", http.StatusText(rec.Code)
		}
		return nil, &apiError{Status: rec.Code, Code: e.Error.Code, Message: e.Error.Message, Details: e.Error.Details}
	}
	if rec.Body.Len() == 0 {
		return map[string]any{"ok": true}, nil
	}
	var out any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("leer respuesta: %w", err)
	}
	return out, nil
}

func (c *call) get(path string, query url.Values) (map[string]any, error) {
	v, err := c.do(http.MethodGet, path, query, nil)
	if err != nil {
		return nil, err
	}
	m, _ := v.(map[string]any)
	return m, nil
}

var keyRe = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}-[0-9]+$`)

// workItem reads a work item by its id or its key, e.g. KAI-42.
func (c *call) workItem(ref string) (map[string]any, error) {
	ref = strings.TrimSpace(ref)
	if _, err := uuid.Parse(ref); err == nil {
		return c.get("/work-items/"+ref, nil)
	}
	if keyRe.MatchString(strings.ToUpper(ref)) {
		return c.get("/work-items/by-key/"+url.PathEscape(strings.ToUpper(ref)), nil)
	}
	return nil, argError("La tarea se indica con su id o su clave (por ejemplo KAI-42): " + ref)
}

// projectID takes a project's id or its identifier, e.g. KAI.
func (c *call) projectID(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if _, err := uuid.Parse(ref); err == nil {
		return ref, nil
	}
	page, err := c.get("/projects", url.Values{"q": {ref}, "page_size": {strconv.Itoa(PageSize)}})
	if err != nil {
		return "", err
	}
	items, _ := page["items"].([]any)
	for _, it := range items {
		if m, ok := it.(map[string]any); ok && strings.EqualFold(str(m["identifier"]), ref) {
			return str(m["id"]), nil
		}
	}
	return "", argError("No encuentro un proyecto con el id o el identificador " + ref + ". Usa search o list_projects.")
}

func id(ref string, what string) (string, error) {
	ref = strings.TrimSpace(ref)
	if _, err := uuid.Parse(ref); err != nil {
		return "", argError(what + " debe ser un id; usa search para encontrarlo.")
	}
	return ref, nil
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

// refID is the id inside a {id, name} reference, or nil.
func refID(v any) any {
	if m, ok := v.(map[string]any); ok {
		return m["id"]
	}
	return nil
}

// shape makes a response readable for the client: documents become
// Markdown (RF-36), each element gets the link to its screen (RF-38), long
// lists are cut (RF-39), and what only the interface uses goes away.
func (c *call) shape(v any, labels map[string]string) any {
	switch x := v.(type) {
	case map[string]any:
		if x["type"] == "doc" {
			b, _ := json.Marshal(x)
			return richtext.ToMarkdown(b, labels)
		}
		if refs, ok := x["refs"].(map[string]any); ok {
			merged := map[string]string{}
			for k, l := range labels {
				merged[k] = l
			}
			for k, r := range refs {
				if m, ok := r.(map[string]any); ok && str(m["label"]) != "" {
					merged[k] = str(m["label"])
				}
			}
			labels = merged
		}
		out := make(map[string]any, len(x)+1)
		for k, val := range x {
			switch k {
			case "refs", "can", "page_version":
				continue
			}
			out[k] = c.shape(val, labels)
			if list, ok := val.([]any); ok && len(list) > c.limit() {
				out[k+"_total"] = len(list)
			}
		}
		key, idv := str(x["key"]), str(x["id"])
		switch {
		case keyRe.MatchString(key) && idv != "":
			out["url"] = c.base + "/work-items/" + key
		case str(x["identifier"]) != "" && str(x["name"]) != "" && idv != "" && x["key"] == nil && x["kind"] == nil:
			out["url"] = c.base + "/projects/" + idv
		}
		return out
	case []any:
		if len(x) > c.limit() {
			x = x[:c.limit()]
		}
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = c.shape(val, labels)
		}
		return out
	}
	return v
}

// args reads the arguments of a call.
type args map[string]any

func (a args) has(k string) bool { _, ok := a[k]; return ok }

func (a args) str(k string) string { return strings.TrimSpace(str(a[k])) }

func (a args) need(k string) (string, error) {
	if v := a.str(k); v != "" {
		return v, nil
	}
	return "", argError("Falta el argumento " + k + ".")
}

// strs accepts a list or a single string.
func (a args) strs(k string) []string {
	switch v := a[k].(type) {
	case string:
		if v = strings.TrimSpace(v); v != "" {
			return []string{v}
		}
	case []any:
		var out []string
		for _, x := range v {
			if s := strings.TrimSpace(str(x)); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func (a args) page() string {
	if n, ok := a["page"].(float64); ok && n >= 1 {
		return strconv.Itoa(int(n))
	}
	return "1"
}

// dateQuery passes the optional cut date as the API's "today".
func (a args) dateQuery() url.Values {
	q := url.Values{}
	if d := a.str("date"); d != "" {
		q.Set("today", d)
	}
	return q
}

// set copies the arguments present into body, renamed by fields (argument → field).
func (a args) set(body map[string]any, fields map[string]string) {
	for arg, field := range fields {
		if a.has(arg) {
			body[field] = a[arg]
		}
	}
}

// Schema pieces.
func prop(typ, desc string) map[string]any { return map[string]any{"type": typ, "description": desc} }

func list(desc string) map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": desc}
}

func enum(desc string, values ...string) map[string]any {
	return map[string]any{"type": "string", "enum": values, "description": desc}
}

func nullable(typ, desc string) map[string]any {
	return map[string]any{"type": []string{typ, "null"}, "description": desc}
}

var (
	pDate      = prop("string", "Fecha de corte AAAA-MM-DD. Por defecto, hoy.")
	pProject   = prop("string", "Id del proyecto o su identificador (por ejemplo KAI).")
	pWorkItem  = prop("string", "Id de la tarea o su clave (por ejemplo KAI-42).")
	pMilestone = prop("string", "Id del hito.")
	pPage      = prop("integer", "Página, desde 1. Cada página trae como máximo 50 elementos.")
	pMarkdown  = prop("string", "Texto en Markdown: títulos, listas, listas de tareas, negrita, cursiva, enlaces, código, citas y tablas.")
)

// workItemFields are the editable fields of a work item, shared by creation and edition.
func workItemFields() map[string]any {
	return map[string]any{
		"title":           prop("string", "Título."),
		"type_id":         prop("string", "Id del tipo (ver project_catalog)."),
		"status_id":       prop("string", "Id del estado. Al editar, solo los de allowed_status_ids de la tarea."),
		"priority_id":     prop("string", "Id de la prioridad."),
		"assignee_ids":    list("Ids de las personas asignadas; reemplaza la lista. [] deja la tarea sin asignar."),
		"start_date":      nullable("string", "Inicio AAAA-MM-DD, o null para quitarlo."),
		"due_date":        nullable("string", "Fin AAAA-MM-DD, o null para quitarlo."),
		"estimated_hours": nullable("number", "Horas estimadas."),
		"size_id":         nullable("string", "Id de la talla, o null para quitarla."),
		"label_ids":       list("Ids de las etiquetas; reemplaza la lista."),
		"parent":          prop("string", "Id o clave de la tarea padre."),
	}
}

var workItemArgs = map[string]string{
	"title": "title", "type_id": "type_id", "status_id": "status_id", "priority_id": "priority_id",
	"assignee_ids": "assignee_ids", "start_date": "start_date", "due_date": "due_date",
	"estimated_hours": "estimated_hours", "size_id": "size_id", "label_ids": "label_ids",
}

func with(m map[string]any, extra map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func tools() []tool {
	return []tool{
		{
			name: "my_dashboard", title: "Mi panel", readOnly: true,
			description: "Mi trabajo: tareas asignadas, atrasadas y próximas, y mis proyectos.",
			params:      map[string]any{"date": pDate},
			run: func(c *call, a args) (any, error) {
				return c.do(http.MethodGet, "/me/dashboard", a.dateQuery(), nil)
			},
		},
		{
			name: "search", title: "Buscar", readOnly: true,
			description: "Busca proyectos, hitos y tareas por texto en su nombre o por clave. Devuelve hasta 10 de cada tipo con su id y su enlace.",
			params:      map[string]any{"query": prop("string", "Texto a buscar.")}, required: []string{"query"},
			run: func(c *call, a args) (any, error) {
				q, err := a.need("query")
				if err != nil {
					return nil, err
				}
				out, err := c.get("/mentions", url.Values{"q": {q}})
				if err != nil {
					return nil, err
				}
				for _, kind := range []string{"work_items", "milestones", "projects", "meetings"} {
					items, _ := out[kind].([]any)
					for _, it := range items {
						m, _ := it.(map[string]any)
						switch kind {
						case "work_items":
							m["key"], _, _ = strings.Cut(str(m["label"]), " ")
						case "milestones":
							m["url"] = c.base + "/milestones/" + str(m["id"])
						case "projects":
							m["url"] = c.base + "/projects/" + str(m["id"])
						case "meetings":
							m["url"] = c.base + "/meetings/" + str(m["id"])
						}
					}
				}
				return out, nil
			},
		},
		{
			name: "list_projects", title: "Listar proyectos", readOnly: true,
			description: "Lista los proyectos que puedo ver, por nombre. Filtra por texto y por ciclo de vida.",
			params: map[string]any{
				"query":     prop("string", "Texto en el nombre o el identificador."),
				"lifecycle": prop("string", "Ciclo de vida, por ejemplo activo, cerrado o archivado."),
				"page":      pPage,
			},
			run: func(c *call, a args) (any, error) {
				q := url.Values{"page": {a.page()}, "page_size": {strconv.Itoa(PageSize)}}
				if v := a.str("query"); v != "" {
					q.Set("q", v)
				}
				if v := a.str("lifecycle"); v != "" {
					q.Set("lifecycle", v)
				}
				return c.do(http.MethodGet, "/projects", q, nil)
			},
		},
		{
			name: "get_project", title: "Ver proyecto", readOnly: true,
			description: "La ficha de un proyecto, con su descripción en Markdown.",
			params:      map[string]any{"project": pProject}, required: []string{"project"},
			run: func(c *call, a args) (any, error) {
				pid, err := c.projectID(a.str("project"))
				if err != nil {
					return nil, err
				}
				return c.do(http.MethodGet, "/projects/"+pid, nil, nil)
			},
		},
		{
			name: "project_overview", title: "Resumen del proyecto", readOnly: true,
			description: "Resumen de un proyecto: avance real frente al plan, brecha, hitos, tareas atrasadas y próximas.",
			params:      map[string]any{"project": pProject, "date": pDate}, required: []string{"project"},
			run: func(c *call, a args) (any, error) {
				pid, err := c.projectID(a.str("project"))
				if err != nil {
					return nil, err
				}
				return c.do(http.MethodGet, "/projects/"+pid+"/overview", a.dateQuery(), nil)
			},
		},
		{
			name: "project_schedule", title: "Plan del proyecto", readOnly: true,
			description: "La línea base y el plan frente al real de cada tarea de un proyecto en una fecha de corte.",
			params:      map[string]any{"project": pProject, "date": pDate}, required: []string{"project"},
			run: func(c *call, a args) (any, error) {
				pid, err := c.projectID(a.str("project"))
				if err != nil {
					return nil, err
				}
				return c.do(http.MethodGet, "/projects/"+pid+"/schedule", a.dateQuery(), nil)
			},
		},
		{
			name: "portfolio", title: "Portafolio", readOnly: true,
			description: "Los programas y sus proyectos con plan, real, brecha y estado en una fecha de corte.",
			params:      map[string]any{"date": pDate},
			run: func(c *call, a args) (any, error) {
				return c.do(http.MethodGet, "/portfolio", a.dateQuery(), nil)
			},
		},
		{
			name: "tracking", title: "Seguimiento", readOnly: true,
			description: "Seguimiento de varios proyectos: indicadores, hitos y tareas atrasadas, en proceso, terminadas y próximas.",
			params: map[string]any{
				"program_id":  prop("string", "Id del programa."),
				"project_ids": list("Ids de los proyectos."),
				"owner_ids":   list("Ids de los responsables de proyecto."),
				"date":        pDate,
			},
			run: func(c *call, a args) (any, error) {
				q := a.dateQuery()
				if v := a.str("program_id"); v != "" {
					q.Set("program_id", v)
				}
				q["project_id"], q["owner_id"] = a.strs("project_ids"), a.strs("owner_ids")
				return c.do(http.MethodGet, "/tracking", q, nil)
			},
		},
		{
			name: "list_milestones", title: "Hitos del proyecto", readOnly: true,
			description: "Los hitos de un proyecto con su fecha, su estado y su avance.",
			params:      map[string]any{"project": pProject}, required: []string{"project"},
			run: func(c *call, a args) (any, error) {
				pid, err := c.projectID(a.str("project"))
				if err != nil {
					return nil, err
				}
				out, err := c.do(http.MethodGet, "/projects/"+pid+"/milestones", nil, nil)
				if err != nil {
					return nil, err
				}
				c.milestoneURLs(out)
				return out, nil
			},
		},
		{
			name: "get_milestone", title: "Ver hito", readOnly: true,
			description: "Un hito con su descripción en Markdown, su avance y sus tareas.",
			params:      map[string]any{"milestone": pMilestone}, required: []string{"milestone"},
			run: func(c *call, a args) (any, error) {
				mid, err := id(a.str("milestone"), "milestone")
				if err != nil {
					return nil, err
				}
				m, err := c.get("/milestones/"+mid, nil)
				if err != nil {
					return nil, err
				}
				items, err := c.do(http.MethodGet, "/milestones/"+mid+"/work-items", nil, nil)
				if err != nil {
					return nil, err
				}
				m["url"], m["work_items"] = c.base+"/milestones/"+mid, items
				return m, nil
			},
		},
		{
			name: "list_work_items", title: "Listar tareas", readOnly: true,
			description: "Las tareas de un proyecto, con filtros. Para ver una tarea completa usa get_work_item.",
			params: map[string]any{
				"project":      pProject,
				"query":        prop("string", "Texto en el título o la clave."),
				"status_ids":   list("Ids de estados."),
				"assignee":     prop("string", "Id de la persona asignada, \"me\" para mí o \"none\" sin asignar."),
				"milestone_id": prop("string", "Id del hito, o \"none\" sin hito."),
				"label_ids":    list("Ids de etiquetas."),
				"closed":       prop("boolean", "true solo cerradas, false solo abiertas."),
				"overdue":      prop("boolean", "true solo atrasadas."),
				"sort":         prop("string", "Orden: key, title, status, due, progress, priority; con - delante, descendente."),
				"page":         pPage,
			},
			required: []string{"project"},
			run: func(c *call, a args) (any, error) {
				pid, err := c.projectID(a.str("project"))
				if err != nil {
					return nil, err
				}
				q := url.Values{"page": {a.page()}, "page_size": {strconv.Itoa(PageSize)}}
				for arg, param := range map[string]string{"query": "q", "assignee": "assignee_id", "milestone_id": "milestone_id", "sort": "sort"} {
					if v := a.str(arg); v != "" {
						q.Set(param, v)
					}
				}
				q["status_id"], q["label_id"] = a.strs("status_ids"), a.strs("label_ids")
				for _, flag := range []string{"closed", "overdue"} {
					if b, ok := a[flag].(bool); ok {
						q.Set(flag, strconv.FormatBool(b))
					}
				}
				return c.do(http.MethodGet, "/projects/"+pid+"/work-items", q, nil)
			},
		},
		{
			name: "get_work_item", title: "Ver tarea", readOnly: true,
			description: "Una tarea completa: campos, descripción en Markdown, estados permitidos, dependencias, subtareas y comentarios.",
			params:      map[string]any{"work_item": pWorkItem}, required: []string{"work_item"},
			run: func(c *call, a args) (any, error) {
				w, err := c.workItem(a.str("work_item"))
				if err != nil {
					return nil, err
				}
				wid := str(w["id"])
				if w["subtasks"], err = c.do(http.MethodGet, "/work-items/"+wid+"/descendants", nil, nil); err != nil {
					return nil, err
				}
				if w["comments"], err = c.do(http.MethodGet, "/work-items/"+wid+"/comments", nil, nil); err != nil {
					return nil, err
				}
				return w, nil
			},
		},
		{
			name: "my_points", title: "Mis puntos", readOnly: true,
			description: "Mis puntos de experiencia, monedas y nivel.",
			params:      map[string]any{},
			run: func(c *call, a args) (any, error) {
				return c.do(http.MethodGet, "/me/points", nil, nil)
			},
		},
		{
			name: "project_catalog", title: "Valores del proyecto", readOnly: true,
			description: "Lo que hace falta para crear o editar tareas en un proyecto: tipos, estados, prioridades, tallas, etiquetas y, si lo gestionas, miembros.",
			params:      map[string]any{"project": pProject}, required: []string{"project"},
			run: func(c *call, a args) (any, error) {
				pid, err := c.projectID(a.str("project"))
				if err != nil {
					return nil, err
				}
				project, err := c.get("/projects/"+pid, nil)
				if err != nil {
					return nil, err
				}
				catalog, err := c.get("/catalog", nil)
				if err != nil {
					return nil, err
				}
				// Only who manages the project sees its members; the rest still
				// needs the types and statuses to move its work items.
				members, err := c.get("/projects/"+pid+"/members", nil)
				var denied *apiError
				if errors.As(err, &denied) && denied.Status == http.StatusForbidden {
					members, err = map[string]any{"items": []any{}}, nil
				}
				if err != nil {
					return nil, err
				}
				allowed := map[string]bool{}
				ids, _ := project["work_item_type_ids"].([]any)
				for _, t := range ids {
					allowed[str(t)] = true
				}
				live := func(key string, keep func(map[string]any) bool) []any {
					all, _ := catalog[key].([]any)
					out := []any{}
					for _, it := range all {
						if m, ok := it.(map[string]any); ok && m["retired"] != true && (keep == nil || keep(m)) {
							out = append(out, it)
						}
					}
					return out
				}
				return map[string]any{
					"work_item_types": live("work_item_types", func(m map[string]any) bool { return len(allowed) == 0 || allowed[str(m["id"])] }),
					"statuses":        live("statuses", nil),
					"priorities":      live("priorities", nil),
					"sizes":           live("sizes", nil),
					"labels":          live("labels", nil),
					"members":         members["items"],
				}, nil
			},
		},
		{
			name: "create_work_item", title: "Crear tarea",
			description: "Crea una tarea o subtarea en un proyecto. Consulta antes project_catalog.",
			params: with(workItemFields(), map[string]any{
				"project":     pProject,
				"description": pMarkdown,
			}),
			required: []string{"project", "type_id", "title"},
			run: func(c *call, a args) (any, error) {
				pid, err := c.projectID(a.str("project"))
				if err != nil {
					return nil, err
				}
				body := map[string]any{}
				a.set(body, workItemArgs)
				if a.has("description") {
					body["description"] = richtext.FromMarkdown(a.str("description"))
				}
				if a.str("parent") != "" {
					parent, err := c.workItem(a.str("parent"))
					if err != nil {
						return nil, err
					}
					body["parent_id"] = parent["id"]
				}
				return c.do(http.MethodPost, "/projects/"+pid+"/work-items", nil, body)
			},
		},
		{
			name: "update_work_item", title: "Editar tarea",
			description: "Cambia campos de una tarea: título, estado, prioridad, asignados, fechas, talla, etiquetas, horas, avance o padre. Solo cambia los que se indican. Para la descripción usa update_page.",
			params: with(workItemFields(), map[string]any{
				"work_item":       pWorkItem,
				"remaining_hours": nullable("number", "Horas restantes."),
				"done_ratio":      prop("integer", "Avance de 0 a 100."),
				"custom_values":   prop("object", "Campos personalizados por id; reemplaza los indicados."),
			}),
			required: []string{"work_item"},
			run: func(c *call, a args) (any, error) {
				w, err := c.workItem(a.str("work_item"))
				if err != nil {
					return nil, err
				}
				var assignees []any
				list, _ := w["assignees"].([]any)
				for _, x := range list {
					assignees = append(assignees, refID(x))
				}
				body := map[string]any{
					"version": w["version"], "type_id": w["type_id"], "status_id": w["status_id"], "priority_id": w["priority_id"],
					"title": w["title"], "description": w["description"], "parent_id": refID(w["parent"]),
					"assignee_ids": assignees, "start_date": w["start_date"], "due_date": w["due_date"],
					"estimated_hours": w["estimated_hours"], "remaining_hours": w["remaining_hours"], "done_ratio": w["done_ratio"],
					"position": w["position"], "custom_values": w["custom_values"], "weight": w["weight"],
					"size_id": w["size_id"], "label_ids": w["label_ids"],
				}
				if assignees == nil {
					body["assignee_ids"] = []any{}
				}
				a.set(body, workItemArgs)
				a.set(body, map[string]string{"remaining_hours": "remaining_hours", "done_ratio": "done_ratio"})
				if extra, ok := a["custom_values"].(map[string]any); ok {
					values, _ := body["custom_values"].(map[string]any)
					merged := map[string]any{}
					for k, v := range values {
						merged[k] = v
					}
					for k, v := range extra {
						merged[k] = v
					}
					body["custom_values"] = merged
				}
				if a.str("parent") != "" {
					parent, err := c.workItem(a.str("parent"))
					if err != nil {
						return nil, err
					}
					body["parent_id"] = parent["id"]
				}
				return c.do(http.MethodPatch, "/work-items/"+str(w["id"]), nil, body)
			},
		},
		{
			name: "add_dependency", title: "Añadir dependencia",
			description: "Relaciona dos tareas: blocks (la primera bloquea a la otra), precedes (va antes), relates o duplicates.",
			params: map[string]any{
				"work_item": pWorkItem,
				"other":     prop("string", "Id o clave de la otra tarea."),
				"type":      enum("Tipo de relación.", "blocks", "precedes", "relates", "duplicates"),
				"lag_days":  prop("integer", "Días de desfase, solo para precedes."),
			},
			required: []string{"work_item", "other", "type"},
			run: func(c *call, a args) (any, error) {
				w, err := c.workItem(a.str("work_item"))
				if err != nil {
					return nil, err
				}
				other, err := c.workItem(a.str("other"))
				if err != nil {
					return nil, err
				}
				body := map[string]any{"to_id": other["id"], "type": a.str("type")}
				a.set(body, map[string]string{"lag_days": "lag_days"})
				return c.do(http.MethodPost, "/work-items/"+str(w["id"])+"/dependencies", nil, body)
			},
		},
		{
			name: "remove_dependency", title: "Quitar dependencia",
			description: "Quita una dependencia entre tareas; su id sale en dependencies de get_work_item. Se puede volver a añadir.",
			params:      map[string]any{"dependency_id": prop("string", "Id de la dependencia.")}, required: []string{"dependency_id"},
			run: func(c *call, a args) (any, error) {
				did, err := id(a.str("dependency_id"), "dependency_id")
				if err != nil {
					return nil, err
				}
				return c.do(http.MethodDelete, "/dependencies/"+did, nil, nil)
			},
		},
		{
			name: "add_comment", title: "Comentar",
			description: "Añade un comentario a una tarea, o responde a un hilo.",
			params: map[string]any{
				"work_item": pWorkItem,
				"text":      pMarkdown,
				"reply_to":  prop("string", "Id del comentario al que se responde."),
			},
			required: []string{"work_item", "text"},
			run: func(c *call, a args) (any, error) {
				w, err := c.workItem(a.str("work_item"))
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
				return c.do(http.MethodPost, "/work-items/"+str(w["id"])+"/comments", nil, body)
			},
		},
		{
			name: "create_milestone", title: "Crear hito",
			description: "Crea un hito en un proyecto.",
			params: map[string]any{
				"project":     pProject,
				"name":        prop("string", "Nombre."),
				"due_date":    nullable("string", "Fecha AAAA-MM-DD."),
				"description": pMarkdown,
			},
			required: []string{"project", "name"},
			run: func(c *call, a args) (any, error) {
				pid, err := c.projectID(a.str("project"))
				if err != nil {
					return nil, err
				}
				body := map[string]any{}
				a.set(body, map[string]string{"name": "name", "due_date": "due_date"})
				if a.has("description") {
					body["description"] = richtext.FromMarkdown(a.str("description"))
				}
				out, err := c.do(http.MethodPost, "/projects/"+pid+"/milestones", nil, body)
				c.milestoneURLs(out)
				return out, err
			},
		},
		{
			name: "update_milestone", title: "Editar hito",
			description: "Cambia el nombre o la fecha de un hito. Para la descripción usa update_page.",
			params: map[string]any{
				"milestone": pMilestone,
				"name":      prop("string", "Nombre."),
				"due_date":  nullable("string", "Fecha AAAA-MM-DD, o null para quitarla."),
			},
			required: []string{"milestone"},
			run: func(c *call, a args) (any, error) {
				mid, err := id(a.str("milestone"), "milestone")
				if err != nil {
					return nil, err
				}
				m, err := c.get("/milestones/"+mid, nil)
				if err != nil {
					return nil, err
				}
				body := map[string]any{
					"version": m["version"], "name": m["name"], "description": m["description"],
					"due_date": m["due_date"], "phase_id": refID(m["phase"]), "position": m["position"],
				}
				a.set(body, map[string]string{"name": "name", "due_date": "due_date"})
				out, err := c.do(http.MethodPatch, "/milestones/"+mid, nil, body)
				c.milestoneURLs(out)
				return out, err
			},
		},
		{
			name: "update_project", title: "Editar proyecto",
			description: "Cambia campos de un proyecto: nombre, responsable, fechas, salud y su explicación. Para la descripción usa update_page.",
			params: map[string]any{
				"project":            pProject,
				"name":               prop("string", "Nombre."),
				"owner_id":           prop("string", "Id del responsable."),
				"start_date":         nullable("string", "Inicio AAAA-MM-DD."),
				"end_date":           nullable("string", "Fin AAAA-MM-DD."),
				"health":             enum("Salud del proyecto.", "en_curso", "en_riesgo", "fuera_de_curso"),
				"health_explanation": prop("string", "Explicación de la salud."),
			},
			required: []string{"project"},
			run: func(c *call, a args) (any, error) {
				pid, err := c.projectID(a.str("project"))
				if err != nil {
					return nil, err
				}
				pr, err := c.get("/projects/"+pid, nil)
				if err != nil {
					return nil, err
				}
				body := map[string]any{
					"version": pr["version"], "name": pr["name"], "description": pr["description"],
					"team_id": refID(pr["team"]), "owner_id": refID(pr["owner"]), "parent_id": refID(pr["parent"]),
					"start_date": pr["start_date"], "end_date": pr["end_date"], "health": pr["health"],
					"health_explanation": pr["health_explanation"], "custom_values": pr["custom_values"],
				}
				a.set(body, map[string]string{
					"name": "name", "owner_id": "owner_id", "start_date": "start_date", "end_date": "end_date",
					"health": "health", "health_explanation": "health_explanation",
				})
				return c.do(http.MethodPatch, "/projects/"+pid, nil, body)
			},
		},
		{
			name: "update_page", title: "Editar descripción",
			description: "Escribe en la descripción (la página) de una tarea, un hito o un proyecto. Por defecto añade el texto al final; con mode=replace la reemplaza entera, y se pierden las imágenes, archivos y menciones que tuviera.",
			params: map[string]any{
				"kind":     enum("De qué es la página: las notas de una reunión son su página.", "work_item", "milestone", "project", "meeting"),
				"id":       prop("string", "Id del elemento; para una tarea vale su clave y para un proyecto su identificador."),
				"markdown": pMarkdown,
				"mode":     enum("append añade al final (por defecto); replace reemplaza.", "append", "replace"),
			},
			required: []string{"kind", "id", "markdown"},
			run: func(c *call, a args) (any, error) {
				var owner map[string]any
				var path string
				var err error
				switch a.str("kind") {
				case "work_item":
					owner, err = c.workItem(a.str("id"))
					path = "/work-items/"
				case "milestone":
					var mid string
					if mid, err = id(a.str("id"), "id"); err == nil {
						owner, err = c.get("/milestones/"+mid, nil)
					}
					path = "/milestones/"
				case "project":
					var pid string
					if pid, err = c.projectID(a.str("id")); err == nil {
						owner, err = c.get("/projects/"+pid, nil)
					}
					path = "/projects/"
				case "meeting":
					var mid string
					if mid, err = id(a.str("id"), "id"); err == nil {
						owner, err = c.get("/meetings/"+mid, nil)
					}
					path = "/meetings/"
				default:
					return nil, argError("kind debe ser work_item, milestone, project o meeting.")
				}
				if err != nil {
					return nil, err
				}
				md, err := a.need("markdown")
				if err != nil {
					return nil, err
				}
				page := richtext.FromMarkdown(md)
				if a.str("mode") != "replace" {
					page = appendDoc(owner["description"], page)
				}
				out, err := c.do(http.MethodPut, path+str(owner["id"])+"/page", nil,
					map[string]any{"page": page, "page_version": owner["page_version"]})
				if err != nil {
					return nil, err
				}
				m, _ := out.(map[string]any)
				return map[string]any{"saved": true, "page": m["page"], "refs": m["refs"]}, nil
			},
		},
	}
}

// milestoneURLs adds the link of each milestone of a milestone or a list.
func (c *call) milestoneURLs(v any) {
	switch x := v.(type) {
	case map[string]any:
		if str(x["id"]) != "" && x["progress"] != nil {
			x["url"] = c.base + "/milestones/" + str(x["id"])
		}
		for _, k := range []string{"items", "milestones"} {
			c.milestoneURLs(x[k])
		}
	case []any:
		for _, it := range x {
			c.milestoneURLs(it)
		}
	}
}

// appendDoc puts the blocks of more after those of the stored page.
func appendDoc(stored any, more json.RawMessage) json.RawMessage {
	doc, ok := stored.(map[string]any)
	if !ok || doc["type"] != "doc" {
		return more
	}
	var extra map[string]any
	if json.Unmarshal(more, &extra) != nil {
		return more
	}
	blocks, _ := doc["content"].([]any)
	// An empty page is one empty paragraph: it gives way to the new text.
	if len(blocks) == 1 {
		if p, ok := blocks[0].(map[string]any); ok && p["type"] == "paragraph" && p["content"] == nil {
			blocks = nil
		}
	}
	added, _ := extra["content"].([]any)
	doc["content"] = append(append([]any{}, blocks...), added...)
	b, _ := json.Marshal(doc)
	return b
}
