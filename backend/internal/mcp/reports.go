package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"openscum/internal/authz"
)

// The read-only tools of the agent that writes project reports (spec 020,
// RF-45 to RF-51). They reuse the internal dispatch of the MCP server, so
// every query has exactly the permissions of the person generating, and a
// guard keeps them inside the tree of the report's project.

// ReportPageSize is the most work items one query returns (RF-51).
const ReportPageSize = 200

// ReportTool is the definition the agent offers to the model.
type ReportTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ReportScope is the tree of the report: the root and whether a project
// belongs to it (RF-48).
type ReportScope struct {
	Root   string
	InTree func(projectID string) bool
}

var errOutsideTree = argError("No tienes acceso a ese proyecto: solo puedes consultar el proyecto del informe y sus subproyectos.")

func (sc ReportScope) guard(projectID string) error {
	if projectID == "" || !sc.InTree(projectID) {
		return errOutsideTree
	}
	return nil
}

type reportTool struct {
	name        string
	description string
	params      map[string]any
	required    []string
	run         func(c *call, sc ReportScope, a args) (any, string, error) // the result and the project it read
}

// ReportTools lists the tools, for the agent request.
func (s *Server) ReportTools() []ReportTool {
	out := make([]ReportTool, 0, len(reportTools))
	for _, t := range reportTools {
		schema := map[string]any{"type": "object", "properties": t.params, "additionalProperties": false}
		if len(t.required) > 0 {
			schema["required"] = t.required
		}
		out = append(out, ReportTool{Name: t.name, Description: t.description, Parameters: schema})
	}
	return out
}

// ErrUnknownReportTool is a name the agent made up.
var ErrUnknownReportTool = errors.New("herramienta desconocida")

// RunReportTool runs one tool as p. A failed query is an answer for the model,
// not an error: err is only for an unknown tool or an internal failure.
// projectID is the project read, for the progress shown to the person (RF-37).
func (s *Server) RunReportTool(ctx context.Context, p *authz.Principal, sc ReportScope, name string, raw map[string]any) (text string, projectID string, err error) {
	for _, t := range reportTools {
		if t.name != name {
			continue
		}
		c := &call{s: s, ctx: ctx, p: p, base: strings.TrimRight(s.conns.publicURL(), "/"), max: ReportPageSize}
		out, pid, err := t.run(c, sc, args(raw))
		if err != nil {
			var ae *apiError
			if errors.As(err, &ae) || errors.As(err, new(argError)) {
				return "Error: " + err.Error(), pid, nil
			}
			return "", pid, err
		}
		b, _ := json.Marshal(c.shape(out, nil))
		return string(b), pid, nil
	}
	return "", "", ErrUnknownReportTool
}

var (
	rProject = prop("string", "Id del proyecto (de project_tree).")
	rDate    = prop("string", "Fecha de corte AAAA-MM-DD. Por defecto, hoy.")
)

// names turns the ids of the catalog into names, so the model reads
// «En curso» and not a uuid; personal data never comes in (RF-50).
type names map[string]string

func (c *call) catalogNames() (names, error) {
	cat, err := c.get("/catalog", nil)
	if err != nil {
		return nil, err
	}
	out := names{}
	for _, key := range []string{"work_item_types", "statuses", "priorities", "sizes", "labels"} {
		list, _ := cat[key].([]any)
		for _, it := range list {
			if m, ok := it.(map[string]any); ok {
				name := str(m["name"])
				if key == "statuses" && m["is_closed"] == true {
					name += " (cerrado)"
				}
				out[str(m["id"])] = name
			}
		}
	}
	return out, nil
}

// statusIDs maps the status names the model asks for to their ids.
func (c *call) statusIDs(wanted []string) ([]string, error) {
	cat, err := c.get("/catalog", nil)
	if err != nil {
		return nil, err
	}
	list, _ := cat["statuses"].([]any)
	var out []string
	for _, w := range wanted {
		found := false
		for _, it := range list {
			if m, ok := it.(map[string]any); ok && strings.EqualFold(str(m["name"]), w) {
				out = append(out, str(m["id"]))
				found = true
			}
		}
		if !found {
			return nil, argError("No hay ningún estado llamado «" + w + "».")
		}
	}
	return out, nil
}

// named replaces the catalog ids of a work item with their names.
func (n names) named(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	for field, to := range map[string]string{"type_id": "type", "status_id": "status", "priority_id": "priority", "size_id": "size"} {
		if id := str(m[field]); id != "" {
			m[to] = n[id]
		}
		delete(m, field)
	}
	if ids, ok := m["label_ids"].([]any); ok {
		labels := make([]string, 0, len(ids))
		for _, id := range ids {
			labels = append(labels, n[str(id)])
		}
		m["labels"] = labels
	}
	delete(m, "label_ids")
	for _, k := range []string{"position", "successors", "allowed_transitions", "custom_fields", "watching", "following"} {
		delete(m, k)
	}
	return m
}

func projectOf(v map[string]any) string {
	if p, ok := v["project"].(map[string]any); ok {
		return str(p["id"])
	}
	return str(v["project_id"])
}

var reportTools = []reportTool{
	{
		name:        "project_tree",
		description: "El proyecto del informe y todos sus subproyectos, con su id, nombre, ciclo de vida, salud y profundidad. Úsala primero.",
		params:      map[string]any{},
		run: func(c *call, sc ReportScope, a args) (any, string, error) {
			out, err := c.do(http.MethodGet, "/projects/"+sc.Root+"/subtree", nil, nil)
			return out, sc.Root, err
		},
	},
	{
		name:        "get_project",
		description: "La ficha de un proyecto del árbol: fechas, responsable, estado y descripción en Markdown.",
		params:      map[string]any{"project": rProject}, required: []string{"project"},
		run: func(c *call, sc ReportScope, a args) (any, string, error) {
			pid := a.str("project")
			if err := sc.guard(pid); err != nil {
				return nil, pid, err
			}
			out, err := c.do(http.MethodGet, "/projects/"+pid, nil, nil)
			return out, pid, err
		},
	},
	{
		name:        "project_overview",
		description: "Resumen de un proyecto del árbol: avance real frente al plan, brecha, semáforo, hitos, tareas atrasadas y próximas.",
		params:      map[string]any{"project": rProject, "date": rDate}, required: []string{"project"},
		run: func(c *call, sc ReportScope, a args) (any, string, error) {
			pid := a.str("project")
			if err := sc.guard(pid); err != nil {
				return nil, pid, err
			}
			out, err := c.do(http.MethodGet, "/projects/"+pid+"/overview", a.dateQuery(), nil)
			return out, pid, err
		},
	},
	{
		name:        "list_milestones",
		description: "Los hitos de un proyecto del árbol, con su fecha, su estado y su avance.",
		params:      map[string]any{"project": rProject}, required: []string{"project"},
		run: func(c *call, sc ReportScope, a args) (any, string, error) {
			pid := a.str("project")
			if err := sc.guard(pid); err != nil {
				return nil, pid, err
			}
			out, err := c.do(http.MethodGet, "/projects/"+pid+"/milestones", nil, nil)
			return out, pid, err
		},
	},
	{
		name:        "get_milestone",
		description: "Un hito con su descripción en Markdown, su avance y sus tareas.",
		params:      map[string]any{"milestone": prop("string", "Id del hito.")}, required: []string{"milestone"},
		run: func(c *call, sc ReportScope, a args) (any, string, error) {
			mid, err := id(a.str("milestone"), "milestone")
			if err != nil {
				return nil, "", err
			}
			m, err := c.get("/milestones/"+mid, nil)
			if err != nil {
				return nil, "", err
			}
			pid := projectOf(m)
			if err := sc.guard(pid); err != nil {
				return nil, "", err
			}
			items, err := c.do(http.MethodGet, "/milestones/"+mid+"/work-items", nil, nil)
			if err != nil {
				return nil, pid, err
			}
			n, err := c.catalogNames()
			if err != nil {
				return nil, pid, err
			}
			if list, ok := items.([]any); ok {
				for i := range list {
					list[i] = n.named(list[i])
				}
			}
			m["work_items"] = items
			return m, pid, nil
		},
	},
	{
		name: "list_work_items",
		description: fmt.Sprintf("Las tareas de un proyecto del árbol, con filtros; como mucho %d por página, y total dice cuántas hay. "+
			"Para leer una tarea completa usa get_work_item.", ReportPageSize),
		params: map[string]any{
			"project":      rProject,
			"query":        prop("string", "Texto en el título o la clave."),
			"statuses":     list("Nombres de estados, por ejemplo «En curso»."),
			"closed":       prop("boolean", "true solo cerradas, false solo abiertas."),
			"overdue":      prop("boolean", "true solo atrasadas: abiertas y con la fecha de fin ya pasada."),
			"blocked":      prop("boolean", "true solo bloqueadas."),
			"milestone_id": prop("string", "Id del hito, o \"none\" sin hito."),
			"assignee_id":  prop("string", "Id de la persona asignada, o \"none\" sin asignar."),
			"due_after":    prop("string", "Fin a partir de esta fecha AAAA-MM-DD, incluida."),
			"due_before":   prop("string", "Fin hasta esta fecha AAAA-MM-DD, incluida."),
			"sort":         prop("string", "Orden: key, title, status, due, progress, priority; con - delante, descendente."),
			"page":         prop("integer", "Página, desde 1."),
		},
		required: []string{"project"},
		run: func(c *call, sc ReportScope, a args) (any, string, error) {
			pid := a.str("project")
			if err := sc.guard(pid); err != nil {
				return nil, pid, err
			}
			q := url.Values{"page": {a.page()}, "page_size": {strconv.Itoa(ReportPageSize)}}
			for _, k := range []string{"milestone_id", "assignee_id", "due_after", "due_before", "sort"} {
				if v := a.str(k); v != "" {
					q.Set(k, v)
				}
			}
			if v := a.str("query"); v != "" {
				q.Set("q", v)
			}
			for _, flag := range []string{"closed", "overdue", "blocked"} {
				if b, ok := a[flag].(bool); ok {
					q.Set(flag, strconv.FormatBool(b))
				}
			}
			if wanted := a.strs("statuses"); len(wanted) > 0 {
				ids, err := c.statusIDs(wanted)
				if err != nil {
					return nil, pid, err
				}
				q["status_id"] = ids
			}
			page, err := c.get("/projects/"+pid+"/work-items", q)
			if err != nil {
				return nil, pid, err
			}
			n, err := c.catalogNames()
			if err != nil {
				return nil, pid, err
			}
			items, _ := page["items"].([]any)
			for i := range items {
				items[i] = n.named(items[i])
			}
			if total, ok := page["total"].(float64); ok {
				page["not_shown"] = max(0, int(total)-len(items))
			}
			return page, pid, nil
		},
	},
	{
		name:        "get_work_item",
		description: "Una tarea de un proyecto del árbol: campos, fechas, avance, asignados y descripción en Markdown.",
		params:      map[string]any{"work_item": prop("string", "Id de la tarea o su clave, por ejemplo KAI-42.")}, required: []string{"work_item"},
		run: func(c *call, sc ReportScope, a args) (any, string, error) {
			w, err := c.workItem(a.str("work_item"))
			if err != nil {
				return nil, "", err
			}
			pid := projectOf(w)
			if err := sc.guard(pid); err != nil {
				return nil, "", err
			}
			n, err := c.catalogNames()
			if err != nil {
				return nil, pid, err
			}
			return n.named(w), pid, nil
		},
	},
}
