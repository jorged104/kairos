package apitest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type mcpConnectionDTO struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	ExpiresAt  string    `json:"expires_at"`
	LastUsedAt *string   `json:"last_used_at"`
	LastClient *string   `json:"last_client"`
	URL        string    `json:"url"`
	User       *struct {
		ID    uuid.UUID `json:"id"`
		Email string    `json:"email"`
	} `json:"user"`
}

type mcpMineDTO struct {
	Enabled bool               `json:"enabled"`
	Items   []mcpConnectionDTO `json:"items"`
}

func enableMCP(t *testing.T, admin *Client, on bool) {
	t.Helper()
	admin.Do(http.MethodPut, "/api/system/settings/mcp", map[string]any{"enabled": on}).Expect(t, http.StatusOK)
}

func connect(t *testing.T, c *Client, name string) mcpConnectionDTO {
	t.Helper()
	var out mcpConnectionDTO
	c.Post("/api/me/mcp-connections", map[string]any{"name": name, "days": 90}).Expect(t, http.StatusCreated).JSON(t, &out)
	return out
}

// mcpReply is what an AI client gets back from one JSON-RPC message.
type mcpReply struct {
	Status int
	Header http.Header
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// rpc posts like an AI client does: no cookie, no Origin, only the URL.
func (e *Env) rpc(t *testing.T, conn mcpConnectionDTO, method string, params any) mcpReply {
	t.Helper()
	path := strings.TrimPrefix(conn.URL, Origin)
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	res, err := http.Post(e.Srv.URL+path, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := mcpReply{Status: res.StatusCode, Header: res.Header}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("%s: %s", err, raw)
		}
	}
	return out
}

type toolResult struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError"`
}

// tool calls a tool and returns its text and whether it failed.
func (e *Env) tool(t *testing.T, conn mcpConnectionDTO, name string, arguments map[string]any) (string, bool) {
	t.Helper()
	r := e.rpc(t, conn, "tools/call", map[string]any{"name": name, "arguments": arguments})
	if r.Status != http.StatusOK || r.Error != nil {
		t.Fatalf("%s = %d %+v", name, r.Status, r.Error)
	}
	var out toolResult
	if err := json.Unmarshal(r.Result, &out); err != nil || len(out.Content) == 0 {
		t.Fatalf("%s: %s", name, r.Result)
	}
	return out.Content[0].Text, out.IsError
}

func (e *Env) toolOK(t *testing.T, conn mcpConnectionDTO, name string, arguments map[string]any) map[string]any {
	t.Helper()
	text, failed := e.tool(t, conn, name, arguments)
	if failed {
		t.Fatalf("%s failed: %s", name, text)
	}
	var out map[string]any
	_ = json.Unmarshal([]byte(text), &out)
	return out
}

func TestMCPConnections(t *testing.T) {
	env := New(t)
	admin := env.Admin()
	anaID := env.CreateUser(UserSpec{Email: "ana@x.com", Roles: []string{"member"}})
	ana, bob := env.As(anaID), env.As(env.CreateUser(UserSpec{Email: "bob@x.com", Roles: []string{"viewer"}}))

	// Off on a new installation (RF-1, RF-4); only the administrator turns it on (RF-2).
	var mine mcpMineDTO
	ana.Get("/api/me/mcp-connections").Expect(t, http.StatusOK).JSON(t, &mine)
	if mine.Enabled || len(mine.Items) != 0 {
		t.Fatalf("mine = %+v", mine)
	}
	ana.Post("/api/me/mcp-connections", map[string]any{"name": "Claude", "days": 90}).Expect(t, http.StatusConflict, "mcp_disabled")
	ana.Do(http.MethodPut, "/api/system/settings/mcp", map[string]any{"enabled": true}).Expect(t, http.StatusForbidden)
	enableMCP(t, admin, true)
	if !system(t, admin).MCP.Enabled {
		t.Fatal("the switch is not on")
	}
	if a := env.AuditActions("mcp"); len(a) != 1 || a[0] != "system.settings_changed" { // RF-7
		t.Fatalf("audit = %v", a)
	}

	// The URL is shown once and only its hash is kept (RF-10 to RF-13).
	ana.Post("/api/me/mcp-connections", map[string]any{"name": "Claude", "days": 7}).Expect(t, http.StatusBadRequest)
	conn := connect(t, ana, "Claude del portátil")
	secret := strings.TrimPrefix(conn.URL, Origin+"/api/mcp/")
	if len(secret) < 43 || secret == conn.URL {
		t.Fatalf("url = %s", conn.URL)
	}
	list := ana.Get("/api/me/mcp-connections").Expect(t, http.StatusOK)
	if strings.Contains(string(list.Body), secret) {
		t.Fatal("the list shows the secret (RF-12)")
	}
	var stored []byte
	if err := env.Store.Pool.QueryRow(context.Background(), "SELECT secret_hash FROM mcp_connections WHERE id = $1", conn.ID).Scan(&stored); err != nil || bytes.Contains(stored, []byte(secret)) {
		t.Fatalf("stored = %x %v", stored, err)
	}
	if a := env.AuditActions(conn.ID.String()); len(a) != 1 || a[0] != "mcp.connection_created" { // RF-25
		t.Fatalf("audit = %v", a)
	}

	// The URL alone is enough, and the client's name is kept (RF-15, RF-19).
	r := env.rpc(t, conn, "initialize", map[string]any{"protocolVersion": "2025-06-18", "clientInfo": map[string]any{"name": "claude-ai"}})
	if r.Status != http.StatusOK || !strings.Contains(string(r.Result), `"protocolVersion":"2025-06-18"`) {
		t.Fatalf("initialize = %d %s", r.Status, r.Result)
	}
	ana.Get("/api/me/mcp-connections").Expect(t, http.StatusOK).JSON(t, &mine)
	if len(mine.Items) != 1 || mine.Items[0].LastUsedAt == nil || mine.Items[0].LastClient == nil || *mine.Items[0].LastClient != "claude-ai" {
		t.Fatalf("mine = %+v", mine.Items)
	}

	// The administrator sees everybody's; nobody else does (RF-23).
	bobConn := connect(t, bob, "ChatGPT")
	var all struct {
		Items []mcpConnectionDTO `json:"items"`
	}
	admin.Get("/api/system/mcp-connections").Expect(t, http.StatusOK).JSON(t, &all)
	if len(all.Items) != 2 || all.Items[0].User == nil {
		t.Fatalf("all = %+v", all.Items)
	}
	ana.Get("/api/system/mcp-connections").Expect(t, http.StatusForbidden)
	ana.Do(http.MethodDelete, "/api/me/mcp-connections/"+bobConn.ID.String(), nil).Expect(t, http.StatusNotFound)

	// Switched off, every request is rejected and the connections stay (RF-3, RF-5, RF-6).
	enableMCP(t, admin, false)
	if r := env.rpc(t, conn, "ping", nil); r.Status != http.StatusForbidden || !strings.Contains(r.Error.Message, "desactivado") {
		t.Fatalf("disabled = %d %+v", r.Status, r.Error)
	}
	enableMCP(t, admin, true)
	if r := env.rpc(t, conn, "ping", nil); r.Status != http.StatusOK {
		t.Fatalf("enabled again = %d", r.Status)
	}

	// Revoked by the administrator, cut at once (RF-24, RF-25).
	admin.Do(http.MethodDelete, "/api/system/mcp-connections/"+bobConn.ID.String(), nil).Expect(t, http.StatusNoContent)
	if r := env.rpc(t, bobConn, "ping", nil); r.Status != http.StatusNotFound {
		t.Fatalf("revoked = %d", r.Status)
	}
	if a := env.AuditActions(bobConn.ID.String()); len(a) != 2 || a[1] != "mcp.connection_revoked" {
		t.Fatalf("audit = %v", a)
	}

	// Expired, unknown or revoked: the same answer (RF-21, RF-22).
	env.Store.Pool.Exec(context.Background(), "UPDATE mcp_connections SET expires_at = now() - interval '1 second' WHERE id = $1", conn.ID)
	expired := env.rpc(t, conn, "ping", nil)
	unknown := env.rpc(t, mcpConnectionDTO{URL: Origin + "/api/mcp/nope"}, "ping", nil)
	if expired.Status != http.StatusNotFound || unknown.Status != http.StatusNotFound || expired.Error.Message != unknown.Error.Message {
		t.Fatalf("expired = %+v, unknown = %+v", expired, unknown)
	}

	// A person revokes their own (RF-20), and has at most 10 (RF-14).
	own := connect(t, ana, "Claude Code")
	ana.Do(http.MethodDelete, "/api/me/mcp-connections/"+own.ID.String(), nil).Expect(t, http.StatusNoContent)
	if r := env.rpc(t, own, "ping", nil); r.Status != http.StatusNotFound {
		t.Fatalf("revoked = %d", r.Status)
	}
	for i := 0; i < 10; i++ {
		connect(t, ana, "c")
	}
	ana.Post("/api/me/mcp-connections", map[string]any{"name": "once", "days": 30}).Expect(t, http.StatusConflict, "mcp_too_many_connections")
}

func TestMCPDeactivatedOwner(t *testing.T) {
	env := New(t)
	admin := env.Admin()
	enableMCP(t, admin, true)
	id := env.CreateUser(UserSpec{Email: "ana@x.com", Roles: []string{"member"}})
	conn := connect(t, env.As(id), "Claude")
	admin.Post("/api/users/"+id.String()+"/deactivate", nil).Expect(t, http.StatusOK)
	if r := env.rpc(t, conn, "ping", nil); r.Status != http.StatusForbidden { // RF-26
		t.Fatalf("deactivated = %d", r.Status)
	}
}

func TestMCPRateLimit(t *testing.T) {
	env := New(t)
	admin := env.Admin()
	enableMCP(t, admin, true)
	env.App.MCP.SetRateLimit(3)
	conn := connect(t, admin, "Claude")
	other := connect(t, admin, "Otra")
	for i := 0; i < 3; i++ {
		env.rpc(t, conn, "ping", nil)
	}
	r := env.rpc(t, conn, "ping", nil)
	if r.Status != http.StatusTooManyRequests || r.Header.Get("Retry-After") == "" { // RF-47
		t.Fatalf("limit = %d %v", r.Status, r.Header)
	}
	if r := env.rpc(t, other, "ping", nil); r.Status != http.StatusOK {
		t.Fatalf("the limit is per connection: %d", r.Status)
	}
}

func TestMCPTools(t *testing.T) {
	s := newWorkSetup(t)
	env := s.env
	enableMCP(t, s.admin, true)
	project := s.project(t, s.lead, map[string]any{"identifier": "MCP", "name": "Kairos IA"})
	lead := connect(t, s.lead, "Claude de Laura")
	viewer := connect(t, s.viewer, "Claude del observador")

	// The tools say what they do and none is irreversible (RF-31 to RF-33).
	var listed struct {
		Tools []struct {
			Name        string `json:"name"`
			Annotations struct {
				ReadOnly    bool `json:"readOnlyHint"`
				Destructive bool `json:"destructiveHint"`
			} `json:"annotations"`
		} `json:"tools"`
	}
	r := env.rpc(t, lead, "tools/list", nil)
	if err := json.Unmarshal(r.Result, &listed); err != nil || len(listed.Tools) < 20 {
		t.Fatalf("tools = %s", r.Result)
	}
	readOnly := map[string]bool{}
	for _, tl := range listed.Tools {
		readOnly[tl.Name] = tl.Annotations.ReadOnly
		for _, banned := range []string{"delete", "archive", "cancel", "close", "user", "team", "catalog_", "system", "audit"} {
			if strings.Contains(tl.Name, banned) || tl.Annotations.Destructive {
				t.Errorf("tool %s", tl.Name)
			}
		}
	}
	if !readOnly["get_work_item"] || readOnly["create_work_item"] {
		t.Fatalf("readOnly = %v", readOnly)
	}

	// The catalog, then a work item written in Markdown (RF-29, RF-30, RF-34, RF-35).
	catalog := env.toolOK(t, lead, "project_catalog", map[string]any{"project": "MCP"})
	if len(catalog["work_item_types"].([]any)) == 0 || len(catalog["members"].([]any)) == 0 {
		t.Fatalf("catalog = %v", catalog)
	}
	// Who does not manage the project gets it too, without the members.
	if c := env.toolOK(t, viewer, "project_catalog", map[string]any{"project": "MCP"}); len(c["statuses"].([]any)) == 0 || len(c["members"].([]any)) != 0 {
		t.Fatalf("viewer catalog = %v", c)
	}
	act := env.CatalogID("work_item_types", "Actividad")
	created := env.toolOK(t, lead, "create_work_item", map[string]any{
		"project": "MCP", "type_id": act.String(), "title": "Preparar la demo",
		"description": "# Pasos\n\n- [ ] conectar\n- [x] probar\n\nVer **notas**.", "assignee_ids": []any{s.leadID.String()},
	})
	key := created["key"].(string)
	if created["url"] != Origin+"/work-items/"+key { // RF-38
		t.Fatalf("created = %v", created)
	}
	if d, _ := created["description"].(string); !strings.HasPrefix(d, "# Pasos") {
		t.Fatalf("the description reads as Markdown (RF-36): %v", created["description"])
	}
	wid := uuid.MustParse(created["id"].(string))
	var wi struct {
		Description json.RawMessage `json:"description"`
	}
	s.lead.Get("/api/work-items/"+wid.String()).Expect(t, http.StatusOK).JSON(t, &wi)
	if !strings.Contains(string(wi.Description), `"taskList"`) {
		t.Fatalf("stored = %s", wi.Description)
	}

	// Editing changes only what it is told, through the work item's flow (RF-30).
	done := env.CatalogID("statuses", "En progreso")
	updated := env.toolOK(t, lead, "update_work_item", map[string]any{"work_item": key, "status_id": done.String(), "due_date": "2026-12-01"})
	if updated["status_id"] != done.String() || updated["title"] != "Preparar la demo" || updated["due_date"] != "2026-12-01" {
		t.Fatalf("updated = %v", updated)
	}
	if as, _ := updated["assignees"].([]any); len(as) != 1 {
		t.Fatalf("the assignees were kept: %v", updated["assignees"])
	}

	// A comment and a page, marked «vía IA» (RF-42 to RF-44).
	env.toolOK(t, lead, "add_comment", map[string]any{"work_item": key, "text": "Hecho desde **Claude**"})
	env.toolOK(t, lead, "update_page", map[string]any{"kind": "work_item", "id": key, "markdown": "Añadido al final."})
	var raw struct {
		Threads []struct {
			Via *struct {
				Name string `json:"name"`
			} `json:"via"`
		} `json:"threads"`
	}
	s.lead.Get("/api/work-items/"+wid.String()+"/comments").Expect(t, http.StatusOK).JSON(t, &raw)
	if len(raw.Threads) != 1 || raw.Threads[0].Via == nil || raw.Threads[0].Via.Name != "Claude de Laura" {
		t.Fatalf("comment via = %+v", raw.Threads)
	}
	var history struct {
		Entries []struct {
			Kind string `json:"kind"`
			Via  *struct {
				Name   string `json:"name"`
				Client string `json:"client"`
			} `json:"via"`
		} `json:"entries"`
	}
	s.lead.Get("/api/work-items/"+wid.String()+"/history").Expect(t, http.StatusOK).JSON(t, &history)
	if len(history.Entries) < 3 {
		t.Fatalf("history = %+v", history.Entries)
	}
	for _, e := range history.Entries {
		if e.Via == nil || e.Via.Name != "Claude de Laura" {
			t.Fatalf("history via = %+v", history.Entries)
		}
	}
	s.lead.Post("/api/work-items/"+wid.String()+"/comments", text("desde la interfaz")).Expect(t, http.StatusCreated)
	s.lead.Get("/api/work-items/"+wid.String()+"/comments").Expect(t, http.StatusOK).JSON(t, &raw)
	if raw.Threads[1].Via != nil {
		t.Fatal("a comment from the interface has no via")
	}

	// Reading back: Markdown, comments and the appended text (RF-29, RF-36).
	got := env.toolOK(t, lead, "get_work_item", map[string]any{"work_item": strings.ToLower(key)})
	desc, _ := got["description"].(string)
	if !strings.Contains(desc, "- [ ] conectar") || !strings.HasSuffix(desc, "Añadido al final.") {
		t.Fatalf("description = %q", desc)
	}
	if b, _ := json.Marshal(got["comments"]); !strings.Contains(string(b), "Hecho desde **Claude**") {
		t.Fatalf("comments = %s", b)
	}

	// Search by name, with the link (RF-37, RF-38).
	found := env.toolOK(t, lead, "search", map[string]any{"query": "demo"})
	if b, _ := json.Marshal(found); !strings.Contains(string(b), Origin+"/work-items/"+key) {
		t.Fatalf("search = %s", b)
	}
	if b, _ := json.Marshal(env.toolOK(t, lead, "list_milestones", map[string]any{"project": project.ID.String()})); !strings.Contains(string(b), "[") {
		t.Fatalf("milestones = %s", b)
	}

	// The same permissions as in the interface (RF-27, RF-28).
	msg, failed := env.tool(t, viewer, "create_work_item", map[string]any{"project": "MCP", "type_id": act.String(), "title": "x"})
	if !failed || !strings.Contains(msg, "forbidden") {
		t.Fatalf("viewer = %v %s", failed, msg)
	}
	if text, failed := env.tool(t, viewer, "get_work_item", map[string]any{"work_item": key}); failed {
		t.Fatalf("the viewer reads: %s", text)
	}
	msg, failed = env.tool(t, lead, "update_work_item", map[string]any{"work_item": key, "due_date": "no-es-fecha"})
	if !failed || !strings.Contains(msg, "validation") && !strings.Contains(msg, "invalid") { // RF-41
		t.Fatalf("invalid = %v %s", failed, msg)
	}
	if r := env.rpc(t, lead, "tools/call", map[string]any{"name": "delete_work_item", "arguments": map[string]any{}}); r.Error == nil {
		t.Fatal("an unknown tool is a protocol error")
	}
}
