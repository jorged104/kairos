package apitest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"openscum/internal/config"
)

// fakeAI is a service compatible with OpenAI's chat API for the connection
// test (spec 020, RF-11 to RF-13).
type fakeAI struct {
	srv    *httptest.Server
	status int
	tools  bool
	auth   atomic.Value // the last Authorization header
}

func newFakeAI(t *testing.T) *fakeAI {
	f := &fakeAI{status: http.StatusOK, tools: true}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.auth.Store(r.Header.Get("Authorization"))
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if f.status != http.StatusOK {
			w.WriteHeader(f.status)
			_, _ = w.Write([]byte(`{"error":{"message":"nope","code":"invalid_api_key"}}`))
			return
		}
		msg := map[string]any{"role": "assistant", "content": "hola"}
		if f.tools {
			msg["tool_calls"] = []any{map[string]any{"id": "1", "type": "function", "function": map[string]any{"name": "ping", "arguments": "{}"}}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": msg}}})
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// fakeAgent plays the agent service (ADR 0004): it runs the script of the
// test, which may call the tools of Kairos, and writes NDJSON lines.
type fakeAgent struct {
	t      *testing.T
	srv    *httptest.Server
	script func(a *agentRun)
	mu     sync.Mutex
	runs   []*agentRun
}

type agentRun struct {
	t     *testing.T
	Req   agentRequest
	w     http.ResponseWriter
	ctx   context.Context
	token string
}

type agentRequest struct {
	LLM          map[string]string `json:"llm"`
	SystemPrompt string            `json:"system_prompt"`
	Prompt       string            `json:"prompt"`
	Tools        []struct {
		Name string `json:"name"`
	} `json:"tools"`
	ToolsURL string `json:"tools_url"`
	Token    string `json:"token"`
}

func newFakeAgent(t *testing.T) *fakeAgent {
	a := &fakeAgent{t: t}
	a.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		run := &agentRun{t: t, w: w, ctx: r.Context()}
		_ = json.NewDecoder(r.Body).Decode(&run.Req)
		run.token = run.Req.Token
		a.mu.Lock()
		a.runs = append(a.runs, run)
		a.mu.Unlock()
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		a.script(run)
	}))
	t.Cleanup(a.srv.Close)
	return a
}

func (a *fakeAgent) last() *agentRun {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.runs[len(a.runs)-1]
}

// tool calls a tool of Kairos as the agent does.
func (r *agentRun) tool(name string, args map[string]any) string {
	return callTool(r.t, r.Req.ToolsURL, r.token, name, args)
}

func callTool(t *testing.T, url, token, name string, args map[string]any) string {
	b, _ := json.Marshal(args)
	req, _ := http.NewRequest(http.MethodPost, url+"/"+name, bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Errorf("tool %s: %v", name, err)
		return ""
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Sprintf("HTTP %d", res.StatusCode)
	}
	var out struct {
		Content string `json:"content"`
	}
	_ = json.NewDecoder(res.Body).Decode(&out)
	return out.Content
}

func (r *agentRun) emit(ev map[string]any) {
	b, _ := json.Marshal(ev)
	_, _ = r.w.Write(append(b, '\n'))
	r.w.(http.Flusher).Flush()
}

type streamEvent struct {
	Type    string `json:"type"`
	Tool    string `json:"tool"`
	Project string `json:"project"`
	Delta   string `json:"delta"`
	Code    string `json:"code"`
	Todos   []struct {
		Content string `json:"content"`
		Status  string `json:"status"`
	} `json:"todos"`
}

func events(t *testing.T, res *Response) []streamEvent {
	t.Helper()
	var out []streamEvent
	sc := bufio.NewScanner(bytes.NewReader(res.Body))
	for sc.Scan() {
		var ev streamEvent
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			t.Fatalf("línea %q: %v", sc.Text(), err)
		}
		if ev.Type != "ping" {
			out = append(out, ev)
		}
	}
	return out
}

func lastEvent(t *testing.T, res *Response) streamEvent {
	t.Helper()
	evs := events(t, res)
	if len(evs) == 0 {
		t.Fatalf("stream vacío: %s", res.Body)
	}
	return evs[len(evs)-1]
}

type reportsSetup struct {
	*workSetup
	agent      *fakeAgent
	ai         *fakeAI
	root, kid  projectDTO
	outside    projectDTO
	pm         *Client
	estadoID   uuid.UUID
	outsideKey string
}

// newReportsSetup builds a program with one subproject, a project outside
// the tree, the AI service configured and the fake agent.
func newReportsSetup(t *testing.T) *reportsSetup {
	t.Helper()
	agent := newFakeAgent(t)
	env := New(t, func(c *config.Config) { c.Agent = config.AgentConfig{URL: agent.srv.URL} })
	env.App.Reports.SetCallbackURL(env.Srv.URL)
	s := &reportsSetup{workSetup: newWorkSetupOn(t, env), agent: agent, ai: newFakeAI(t)}
	s.root = s.project(t, s.lead, map[string]any{"identifier": "PRG", "name": "Programa", "owner_id": s.memberID})
	s.kid = s.project(t, s.lead, map[string]any{"identifier": "SUB", "name": "Subproyecto", "parent_id": s.root.ID})
	s.outside = s.project(t, s.lead, map[string]any{"identifier": "FUE", "name": "Fuera"})
	s.pm = s.member // the PM of the program (RF-34)
	s.admin.Do(http.MethodPut, "/api/system/settings/ai", map[string]any{
		"enabled": true, "base_url": s.ai.srv.URL + "/v1", "api_key": "sk-secreta", "model": "modelo-x",
	}).Expect(t, http.StatusOK)
	var templates struct {
		Items []struct {
			ID   uuid.UUID `json:"id"`
			Name string    `json:"name"`
		} `json:"items"`
	}
	s.pm.Get("/api/report-templates").Expect(t, http.StatusOK).JSON(t, &templates)
	for _, tm := range templates.Items {
		if tm.Name == "Estado del proyecto" {
			s.estadoID = tm.ID
		}
	}
	if s.estadoID == uuid.Nil {
		t.Fatalf("falta la plantilla sembrada (RF-23): %+v", templates)
	}
	nuevo := env.CatalogID("statuses", "Nuevo")
	env.SeedWorkItem(s.kid.ID, nuevo, s.leadID)
	var key string
	wi := env.SeedWorkItem(s.outside.ID, nuevo, s.leadID)
	_ = env.Store.Pool.QueryRow(context.Background(), "SELECT 'FUE-' || sequence_number FROM work_items WHERE id = $1", wi).Scan(&key)
	s.outsideKey = key
	return s
}

func (s *reportsSetup) generate(c *Client, project uuid.UUID, body map[string]any) *Response {
	return c.Post("/api/projects/"+project.String()+"/reports/generate", body)
}

func TestReportAISettings(t *testing.T) {
	s := newWorkSetup(t)
	ai := newFakeAI(t)
	body := map[string]any{"enabled": true, "base_url": ai.srv.URL + "/v1", "api_key": "sk-secreta", "model": "modelo-x"}

	// Only the system administrator (RF-1); switched on, it needs an address and a model.
	s.member.Do(http.MethodPut, "/api/system/settings/ai", body).Expect(t, http.StatusForbidden, "forbidden")
	s.admin.Do(http.MethodPut, "/api/system/settings/ai", map[string]any{"enabled": true, "base_url": "", "model": "m"}).
		Expect(t, http.StatusBadRequest, "validation_failed")
	s.admin.Post("/api/system/settings/ai/test", nil).Expect(t, http.StatusConflict, "ai_not_configured")

	// A new installation has it off (RF-2); the key never comes back (RF-8).
	var view struct {
		AI struct {
			Enabled        bool   `json:"enabled"`
			BaseURL        string `json:"base_url"`
			Model          string `json:"model"`
			APIKeySet      bool   `json:"api_key_set"`
			AgentAvailable bool   `json:"agent_available"`
			Ready          bool   `json:"ready"`
		} `json:"ai"`
	}
	s.admin.Get("/api/system/settings").Expect(t, http.StatusOK).JSON(t, &view)
	if view.AI.Enabled || view.AI.APIKeySet {
		t.Fatalf("ia de serie = %+v", view.AI)
	}
	res := s.admin.Do(http.MethodPut, "/api/system/settings/ai", body).Expect(t, http.StatusOK)
	res.JSON(t, &view)
	if !view.AI.Enabled || !view.AI.APIKeySet || view.AI.Model != "modelo-x" || strings.Contains(string(res.Body), "sk-secreta") {
		t.Fatalf("ia = %s", res.Body)
	}
	// Without the agent service there is no generation (ADR 0004).
	if view.AI.AgentAvailable || view.AI.Ready {
		t.Fatalf("sin agente no está lista: %+v", view.AI)
	}

	// The test works with the saved key (RF-11, RF-12); an empty key keeps it (RF-9).
	s.admin.Post("/api/system/settings/ai/test", nil).Expect(t, http.StatusNoContent)
	if got := ai.auth.Load(); got != "Bearer sk-secreta" {
		t.Fatalf("Authorization = %v", got)
	}
	body["api_key"] = ""
	body["model"] = "modelo-y"
	s.admin.Do(http.MethodPut, "/api/system/settings/ai", body).Expect(t, http.StatusOK)
	s.admin.Post("/api/system/settings/ai/test", nil).Expect(t, http.StatusNoContent)
	if got := ai.auth.Load(); got != "Bearer sk-secreta" {
		t.Fatalf("la clave no se conservó: %v", got)
	}

	// The failures say where (RF-13).
	ai.status = http.StatusUnauthorized
	res = s.admin.Post("/api/system/settings/ai/test", nil).Expect(t, http.StatusBadRequest, "ai_test_failed")
	if !strings.Contains(string(res.Body), `"step":"auth"`) {
		t.Fatalf("paso = %s", res.Body)
	}
	ai.status, ai.tools = http.StatusOK, false
	res = s.admin.Post("/api/system/settings/ai/test", nil).Expect(t, http.StatusBadRequest, "ai_test_failed")
	if !strings.Contains(string(res.Body), `"step":"tools"`) {
		t.Fatalf("paso = %s", res.Body)
	}
	body["base_url"] = "http://127.0.0.1:1/v1"
	s.admin.Do(http.MethodPut, "/api/system/settings/ai", body).Expect(t, http.StatusOK)
	res = s.admin.Post("/api/system/settings/ai/test", nil).Expect(t, http.StatusBadRequest, "ai_test_failed")
	if !strings.Contains(string(res.Body), `"step":"connect"`) {
		t.Fatalf("paso = %s", res.Body)
	}

	// Audited, the key only as changed (RF-15).
	var audits []string
	rows, err := s.env.Store.Pool.Query(context.Background(), "SELECT after::text FROM audit_log WHERE action = 'system.settings_changed' AND target_id = 'ai' ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var a string
		_ = rows.Scan(&a)
		audits = append(audits, a)
	}
	rows.Close()
	if len(audits) != 3 || strings.Contains(strings.Join(audits, ""), "sk-secreta") || !strings.Contains(audits[0], `"ai_api_key": "changed:`) {
		t.Fatalf("auditoría = %v", audits)
	}
}

func TestReportTemplates(t *testing.T) {
	s := newWorkSetup(t)
	type tmpl struct {
		ID       uuid.UUID `json:"id"`
		Name     string    `json:"name"`
		Prompt   string    `json:"prompt"`
		Archived bool      `json:"archived"`
	}
	var list struct {
		Items []tmpl `json:"items"`
	}
	// The neutral template comes with the installation (RF-23); generators do not see prompts.
	s.member.Get("/api/report-templates").Expect(t, http.StatusOK).JSON(t, &list)
	if len(list.Items) != 1 || list.Items[0].Name != "Estado del proyecto" || list.Items[0].Prompt != "" {
		t.Fatalf("plantillas = %+v", list.Items)
	}
	s.member.Get("/api/system/report-templates").Expect(t, http.StatusForbidden, "forbidden")
	s.member.Post("/api/system/report-templates", map[string]any{"name": "X", "prompt": "p"}).Expect(t, http.StatusForbidden, "forbidden")

	var weekly tmpl
	s.admin.Post("/api/system/report-templates", map[string]any{"name": "Estado semanal", "description": "Cada lunes", "prompt": "Resume la semana."}).
		Expect(t, http.StatusCreated).JSON(t, &weekly)
	// Unique among the active ones, ignoring case (RF-21); the prompt is required (RF-22).
	s.admin.Post("/api/system/report-templates", map[string]any{"name": " estado SEMANAL ", "prompt": "x"}).
		Expect(t, http.StatusConflict, "report_template_name_taken")
	s.admin.Post("/api/system/report-templates", map[string]any{"name": "Vacía", "prompt": "  "}).Expect(t, http.StatusBadRequest, "validation_failed")

	var updated tmpl
	s.admin.Do(http.MethodPut, "/api/system/report-templates/"+weekly.ID.String(), map[string]any{"name": "Estado semanal", "prompt": "Otro prompt."}).
		Expect(t, http.StatusOK).JSON(t, &updated)
	if updated.Prompt != "Otro prompt." {
		t.Fatalf("actualizada = %+v", updated)
	}
	var archived tmpl
	s.admin.Post("/api/system/report-templates/"+weekly.ID.String()+"/archive", nil).Expect(t, http.StatusOK).JSON(t, &archived)
	if !archived.Archived {
		t.Fatalf("archivada = %+v", archived)
	}
	// Archived, it is not offered (RF-25) and the name is free again.
	s.member.Get("/api/report-templates").Expect(t, http.StatusOK).JSON(t, &list)
	if len(list.Items) != 1 {
		t.Fatalf("activas = %+v", list.Items)
	}
	s.admin.Post("/api/system/report-templates", map[string]any{"name": "Estado semanal", "prompt": "Nueva."}).Expect(t, http.StatusCreated)

	// Audited, the prompt only as changed (RF-27).
	if got := s.env.AuditActions(weekly.ID.String()); strings.Join(got, ",") != "report_template.created,report_template.updated,report_template.archived" {
		t.Fatalf("auditoría = %v", got)
	}
	var after string
	_ = s.env.Store.Pool.QueryRow(context.Background(), "SELECT after::text FROM audit_log WHERE target_id = $1 AND action = 'report_template.updated'", weekly.ID.String()).Scan(&after)
	if strings.Contains(after, "Otro prompt") || !strings.Contains(after, "prompt_changed") {
		t.Fatalf("auditoría = %s", after)
	}
}

type reportDTO struct {
	ID           uuid.UUID       `json:"id"`
	Title        string          `json:"title"`
	TemplateName string          `json:"template_name"`
	Content      string          `json:"content"`
	Doc          json.RawMessage `json:"doc"`
	Version      int32           `json:"version"`
	Author       struct {
		Name string `json:"name"`
	} `json:"author"`
	Can struct {
		Manage   bool `json:"manage"`
		Generate bool `json:"generate"`
	} `json:"can"`
}

func TestReports(t *testing.T) {
	s := newWorkSetup(t)
	p := s.project(t, s.lead, map[string]any{"identifier": "INF", "name": "Informes"})
	path := "/api/projects/" + p.ID.String() + "/reports"
	content := "# Estado\n\nTodo **bien**.\n\n<script>alert(1)</script>\n\n| A | B |\n|---|---|\n| 1 | 2 |\n"

	// Who manages the project writes (RF-34); a participant does not.
	s.member.Post(path, map[string]any{"title": "No", "content": "x", "origin": "manual"}).Expect(t, http.StatusForbidden, "forbidden")
	s.lead.Post(path, map[string]any{"title": "", "content": "x", "origin": "manual"}).Expect(t, http.StatusBadRequest, "validation_failed")
	s.lead.Post(path, map[string]any{"title": "T", "content": " ", "origin": "manual"}).Expect(t, http.StatusBadRequest, "validation_failed")

	var r reportDTO
	s.lead.Post(path, map[string]any{"title": "Semana 1", "content": content, "origin": "ai"}).Expect(t, http.StatusCreated).JSON(t, &r)
	if r.Author.Name != "Laura" || r.Version != 1 || !r.Can.Manage || r.Can.Generate {
		t.Fatalf("informe = %+v", r)
	}

	// Everyone who sees the project reads it (RF-28, RF-31); HTML is only text (RF-32).
	var list struct {
		Items []reportDTO `json:"items"`
		Can   struct {
			Manage bool `json:"manage"`
		} `json:"can"`
	}
	s.viewer.Get(path).Expect(t, http.StatusOK).JSON(t, &list)
	if len(list.Items) != 1 || list.Can.Manage {
		t.Fatalf("lista = %+v", list)
	}
	var read reportDTO
	s.viewer.Get("/api/reports/"+r.ID.String()).Expect(t, http.StatusOK).JSON(t, &read)
	doc := string(read.Doc)
	if !strings.Contains(doc, `"heading"`) || !strings.Contains(doc, `"table"`) || !strings.Contains(doc, `"bold"`) ||
		!strings.Contains(doc, `alert(1)`) || strings.Contains(doc, `"html"`) {
		t.Fatalf("doc = %s", doc)
	}

	// Overwritten, with a version (RF-63, RF-64).
	update := map[string]any{"title": "Semana 1 bis", "content": "Nuevo", "origin": "ai_edited", "version": 1}
	s.member.Do(http.MethodPut, "/api/reports/"+r.ID.String(), update).Expect(t, http.StatusForbidden, "forbidden")
	s.lead.Do(http.MethodPut, "/api/reports/"+r.ID.String(), update).Expect(t, http.StatusOK).JSON(t, &read)
	if read.Version != 2 || read.Content != "Nuevo" || read.Title != "Semana 1 bis" {
		t.Fatalf("guardado = %+v", read)
	}
	s.admin.Do(http.MethodPut, "/api/reports/"+r.ID.String(), update).Expect(t, http.StatusConflict, "version_conflict")

	// The preview renders a draft the same way.
	var preview struct {
		Doc json.RawMessage `json:"doc"`
	}
	s.member.Post("/api/reports/preview", map[string]any{"content": "## Hola"}).Expect(t, http.StatusOK).JSON(t, &preview)
	if !strings.Contains(string(preview.Doc), `"heading"`) {
		t.Fatalf("vista previa = %s", preview.Doc)
	}

	s.member.Do(http.MethodDelete, "/api/reports/"+r.ID.String(), nil).Expect(t, http.StatusForbidden, "forbidden")
	s.lead.Do(http.MethodDelete, "/api/reports/"+r.ID.String(), nil).Expect(t, http.StatusNoContent)
	s.lead.Get("/api/reports/"+r.ID.String()).Expect(t, http.StatusNotFound, "not_found")

	// Audited with the origin, the content only as changed (RF-69, RF-70).
	if got := s.env.AuditActions(r.ID.String()); strings.Join(got, ",") != "report.created,report.updated,report.deleted" {
		t.Fatalf("auditoría = %v", got)
	}
	var after string
	_ = s.env.Store.Pool.QueryRow(context.Background(), "SELECT after::text FROM audit_log WHERE target_id = $1 AND action = 'report.updated'", r.ID.String()).Scan(&after)
	if !strings.Contains(after, `"origin": "ai_edited"`) || !strings.Contains(after, "content_changed") || strings.Contains(after, "Nuevo") {
		t.Fatalf("auditoría = %s", after)
	}
}

func TestReportGeneration(t *testing.T) {
	s := newReportsSetup(t)
	var outsideAnswer, workItemAnswer, treeAnswer, kidAnswer string
	s.agent.script = func(a *agentRun) {
		treeAnswer = a.tool("project_tree", nil)
		kidAnswer = a.tool("list_work_items", map[string]any{"project": s.kid.ID.String()})
		outsideAnswer = a.tool("get_project", map[string]any{"project": s.outside.ID.String()})
		workItemAnswer = a.tool("get_work_item", map[string]any{"work_item": s.outsideKey})
		a.emit(map[string]any{"type": "todos", "todos": []any{map[string]any{"content": "Leer el árbol", "status": "completed"}}})
		a.emit(map[string]any{"type": "text", "delta": "Voy a consultar"})
		a.emit(map[string]any{"type": "reset"})
		a.emit(map[string]any{"type": "text", "delta": "# Informe\n"})
		a.emit(map[string]any{"type": "text", "delta": "Todo bien."})
		a.emit(map[string]any{"type": "done"})
	}

	// Only who manages the project generates (RF-34).
	s.outsider.Post("/api/projects/"+s.root.ID.String()+"/reports/generate", map[string]any{"template_id": s.estadoID}).
		Expect(t, http.StatusForbidden, "forbidden")
	s.generate(s.pm, s.root.ID, map[string]any{}).Expect(t, http.StatusBadRequest, "validation_failed")

	res := s.generate(s.pm, s.root.ID, map[string]any{"template_id": s.estadoID, "instructions": "Destaca los riesgos."}).Expect(t, http.StatusOK)
	if ct := res.Header.Get("Content-Type"); ct != "application/x-ndjson" {
		t.Fatalf("content-type = %s", ct)
	}
	evs := events(t, res)
	var types []string
	var tools []string
	var todos streamEvent
	for _, ev := range evs {
		switch ev.Type {
		case "tool": // relayed as they happen, so they may come between the lines
			tools = append(tools, ev.Tool+"@"+ev.Project)
		case "todos":
			todos = ev
			types = append(types, ev.Type)
		default:
			types = append(types, ev.Type)
		}
	}
	// The progress names the project read (RF-37); the plan (RF-38); text, reset, done.
	if strings.Join(tools, ",") != "project_tree@Programa,list_work_items@Subproyecto,get_project@,get_work_item@" {
		t.Fatalf("herramientas = %v", tools)
	}
	if got := strings.Join(types, ","); got != "todos,text,reset,text,text,done" {
		t.Fatalf("eventos = %v", types)
	}
	if len(todos.Todos) != 1 || todos.Todos[0].Content != "Leer el árbol" {
		t.Fatalf("todos = %+v", todos)
	}

	// What the agent got: the fixed rules over the template (RF-53), the
	// instructions (RF-36), the AI service and only read tools (RF-49).
	req := s.agent.last().Req
	if !strings.Contains(req.SystemPrompt, "No inventes cifras") || !strings.Contains(req.SystemPrompt, "avance real frente al plan") ||
		!strings.Contains(req.Prompt, "Destaca los riesgos.") || !strings.Contains(req.Prompt, "Programa") {
		t.Fatalf("petición = %+v", req)
	}
	if req.LLM["api_key"] != "sk-secreta" || req.LLM["model"] != "modelo-x" || len(req.Tools) != 7 {
		t.Fatalf("petición = %+v", req)
	}
	// The tree, its work items with names, nothing outside, no emails (RF-45 to RF-50).
	if !strings.Contains(treeAnswer, "Subproyecto") || strings.Contains(treeAnswer, "Fuera") {
		t.Fatalf("árbol = %s", treeAnswer)
	}
	if !strings.Contains(kidAnswer, `"status":"Nuevo"`) || strings.Contains(kidAnswer, "status_id") || strings.Contains(kidAnswer, "@x.com") {
		t.Fatalf("tareas = %s", kidAnswer)
	}
	if !strings.Contains(outsideAnswer, "No tienes acceso") || !strings.Contains(workItemAnswer, "No tienes acceso") || strings.Contains(workItemAnswer, "seed") {
		t.Fatalf("fuera del árbol = %q / %q", outsideAnswer, workItemAnswer)
	}
	// The token dies with the generation.
	if got := callTool(t, s.env.Srv.URL+"/api/reports/agent/tools", s.agent.last().token, "project_tree", nil); got != "HTTP 401" {
		t.Fatalf("token tras terminar = %q", got)
	}

	// A change sends the draft and the instruction (RF-56).
	s.generate(s.pm, s.root.ID, map[string]any{"draft": "# Borrador"}).Expect(t, http.StatusBadRequest, "validation_failed")
	res = s.generate(s.pm, s.root.ID, map[string]any{"draft": "# Borrador", "change": "Añade riesgos"}).Expect(t, http.StatusOK)
	if lastEvent(t, res).Type != "done" || !strings.Contains(s.agent.last().Req.Prompt, "# Borrador") || !strings.Contains(s.agent.last().Req.Prompt, "Añade riesgos") {
		t.Fatalf("cambio = %s / %s", res.Body, s.agent.last().Req.Prompt)
	}
}

func TestReportGenerationLimits(t *testing.T) {
	s := newReportsSetup(t)

	// 200 work items per query, and how many were left out (RF-51).
	nuevo := s.env.CatalogID("statuses", "Nuevo")
	for range 204 {
		s.env.SeedWorkItem(s.kid.ID, nuevo, s.leadID)
	}
	var page struct {
		Items    []any `json:"items"`
		Total    int   `json:"total"`
		NotShown int   `json:"not_shown"`
	}
	var limitAnswer string
	s.agent.script = func(a *agentRun) {
		_ = json.Unmarshal([]byte(a.tool("list_work_items", map[string]any{"project": s.kid.ID.String()})), &page)
		for range 59 {
			a.tool("project_tree", nil)
		}
		limitAnswer = a.tool("project_tree", nil) // the 61st
		a.emit(map[string]any{"type": "text", "delta": "ok"})
		a.emit(map[string]any{"type": "done"})
	}
	s.generate(s.pm, s.root.ID, map[string]any{"template_id": s.estadoID}).Expect(t, http.StatusOK)
	if len(page.Items) != 200 || page.Total != 205 || page.NotShown != 5 {
		t.Fatalf("página = %d de %d, fuera %d", len(page.Items), page.Total, page.NotShown)
	}
	if !strings.Contains(limitAnswer, "límite de 60 consultas") {
		t.Fatalf("consulta 61 = %q", limitAnswer)
	}

	// An empty text is no report (RF-76); the errors of the AI come through (RF-73).
	s.agent.script = func(a *agentRun) { a.emit(map[string]any{"type": "done"}) }
	if ev := lastEvent(t, s.generate(s.pm, s.root.ID, map[string]any{"template_id": s.estadoID})); ev.Code != "ai_empty" {
		t.Fatalf("vacío = %+v", ev)
	}
	s.agent.script = func(a *agentRun) {
		a.emit(map[string]any{"type": "text", "delta": "medio"})
		a.emit(map[string]any{"type": "error", "code": "ai_quota"})
	}
	if ev := lastEvent(t, s.generate(s.pm, s.root.ID, map[string]any{"template_id": s.estadoID})); ev.Code != "ai_quota" {
		t.Fatalf("cuota = %+v", ev)
	}
	s.agent.script = func(a *agentRun) { a.emit(map[string]any{"type": "error", "code": "algo-raro"}) }
	if ev := lastEvent(t, s.generate(s.pm, s.root.ID, map[string]any{"template_id": s.estadoID})); ev.Code != "ai_error" {
		t.Fatalf("desconocido = %+v", ev)
	}

	// Ten minutes at most (RF-72), here shortened.
	s.env.App.Reports.SetMaxTime(300 * time.Millisecond)
	s.agent.script = func(a *agentRun) { <-a.ctx.Done() }
	if ev := lastEvent(t, s.generate(s.pm, s.root.ID, map[string]any{"template_id": s.estadoID})); ev.Code != "ai_too_long" {
		t.Fatalf("tiempo = %+v", ev)
	}
	s.env.App.Reports.SetMaxTime(time.Minute)

	// Two at a time per person (RF-77).
	release := make(chan struct{})
	started := make(chan struct{}, 2)
	s.agent.script = func(a *agentRun) {
		started <- struct{}{}
		select {
		case <-release:
		case <-a.ctx.Done():
		}
		a.emit(map[string]any{"type": "text", "delta": "ok"})
		a.emit(map[string]any{"type": "done"})
	}
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.env.As(s.memberID).Post("/api/projects/"+s.root.ID.String()+"/reports/generate", map[string]any{"template_id": s.estadoID})
		}()
	}
	<-started
	<-started
	s.generate(s.pm, s.root.ID, map[string]any{"template_id": s.estadoID}).Expect(t, http.StatusTooManyRequests, "report_generation_busy")
	close(release)
	wg.Wait()

	// Switched off, no generation (RF-16), but the reports are still readable and editable (RF-17).
	s.admin.Do(http.MethodPut, "/api/system/settings/ai", map[string]any{"enabled": false, "base_url": s.ai.srv.URL + "/v1", "model": "modelo-x"}).
		Expect(t, http.StatusOK)
	s.generate(s.pm, s.root.ID, map[string]any{"template_id": s.estadoID}).Expect(t, http.StatusConflict, "ai_not_configured")
	var list struct {
		Can struct {
			Manage   bool `json:"manage"`
			Generate bool `json:"generate"`
		} `json:"can"`
	}
	s.pm.Get("/api/projects/"+s.root.ID.String()+"/reports").Expect(t, http.StatusOK).JSON(t, &list)
	if !list.Can.Manage || list.Can.Generate {
		t.Fatalf("can = %+v", list.Can)
	}
}
