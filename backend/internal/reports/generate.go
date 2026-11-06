package reports

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"openscum/internal/apperr"
	"openscum/internal/authz"
	"openscum/internal/llm"
	"openscum/internal/mcp"
	"openscum/internal/projects"
)

// Limits of a generation (spec 020).
const (
	MaxQueries     = 60               // RF-52
	MaxGeneration  = 10 * time.Minute // RF-72
	MaxPerPerson   = 2                // RF-77
	PingEvery      = 10 * time.Second // keeps proxies from closing a quiet stream
	maxInstruction = 2000             // RF-35, RF-56
	maxDraft       = 200000           // RF-67
)

var (
	errAINotConfigured = apperr.New(http.StatusConflict, "ai_not_configured", "La generación de informes no está disponible")
	errBusy            = apperr.New(http.StatusTooManyRequests, "report_generation_busy", "Ya tienes dos informes generándose")
)

// Rules are the fixed instructions above the template's prompt, which the
// administrator cannot edit (RF-53).
const Rules = `Eres un asistente que escribe informes de proyecto para Kairos, una herramienta de gestión de proyectos.

Reglas que siempre cumples:
- Escribe el informe solo en Markdown: títulos con #, listas, tablas, negrita, cursiva, enlaces y citas. Nunca HTML.
- Usa solo datos que hayas obtenido con las herramientas. No inventes cifras, fechas, personas ni tareas; si un dato no está, dilo.
- Antes de escribir, planifica tus pasos con write_todos y consulta lo que necesites: empieza por project_tree.
- Solo puedes leer el proyecto del informe y sus subproyectos. No puedes cambiar nada.
- Escribe en español, salvo que las instrucciones del tipo de informe pidan otro idioma.
- Tu respuesta final es el informe completo y nada más: sin saludos, sin explicar lo que has hecho y sin envolverlo en un bloque de código.`

// Generator runs the generations: it opens a session per generation, calls
// the agent service and relays what happens to the browser as NDJSON lines.
type Generator struct {
	svc         *Service
	tools       *mcp.Server
	callbackURL func() string
	client      *http.Client
	logger      *slog.Logger
	now         func() time.Time
	maxTime     time.Duration

	mu       sync.Mutex
	sessions map[string]*session // by the hash of the token
	running  map[uuid.UUID]int
}

func NewGenerator(svc *Service, tools *mcp.Server, callbackURL func() string, logger *slog.Logger) *Generator {
	return &Generator{
		svc: svc, tools: tools, callbackURL: callbackURL, logger: logger,
		// One connection per generation: a stream is never reused.
		client: &http.Client{Transport: &http.Transport{Proxy: http.ProxyFromEnvironment, DisableKeepAlives: true}}, now: time.Now, maxTime: MaxGeneration,
		sessions: map[string]*session{}, running: map[uuid.UUID]int{},
	}
}

// SetMaxTime shortens the limit, for the tests.
func (g *Generator) SetMaxTime(d time.Duration) { g.maxTime = d }

// SetCallbackURL changes how the agent reaches the API, for the tests.
func (g *Generator) SetCallbackURL(url string) { g.callbackURL = func() string { return url } }

// session is one generation: who generates, the tree it may read and the
// queries made so far.
type session struct {
	p      *authz.Principal
	scope  mcp.ReportScope
	names  map[string]string
	events chan Event

	mu      sync.Mutex
	queries int
}

// Event is one line of the stream to the browser.
type Event struct {
	Type    string `json:"type"` // todos, tool, text, reset, ping, done, error
	Todos   any    `json:"todos,omitempty"`
	Tool    string `json:"tool,omitempty"`
	Project string `json:"project,omitempty"`
	Delta   string `json:"delta,omitempty"`
	Code    string `json:"code,omitempty"`
}

type generateRequest struct {
	TemplateID   *uuid.UUID `json:"template_id"`
	Instructions string     `json:"instructions"`
	Draft        string     `json:"draft"`
	Change       string     `json:"change"`
}

func (r *generateRequest) Normalize() {
	r.Instructions, r.Change = strings.TrimSpace(r.Instructions), strings.TrimSpace(r.Change)
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (g *Generator) acquire(user uuid.UUID) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.running[user] >= MaxPerPerson {
		return false
	}
	g.running[user]++
	return true
}

func (g *Generator) release(user uuid.UUID) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.running[user]--; g.running[user] <= 0 {
		delete(g.running, user)
	}
}

// prepared is a checked request, ready to stream.
type prepared struct {
	project string
	system  string
	prompt  string
	scope   mcp.ReportScope
	names   map[string]string
}

// prepare checks permissions, configuration and input before any byte of the
// stream, so these failures are ordinary JSON errors.
func (g *Generator) prepare(ctx context.Context, p *authz.Principal, projectID uuid.UUID, req generateRequest) (prepared, error) {
	q := g.svc.store.Queries
	project, err := projects.Get(ctx, q, p, projectID, false)
	if err != nil {
		return prepared{}, err
	}
	if !p.CanManageProject(projects.Ref(project)) {
		return prepared{}, apperr.ErrForbidden // RF-34
	}
	if !g.svc.Ready() {
		return prepared{}, errAINotConfigured // RF-16
	}
	change := req.Draft != "" || req.Change != ""
	switch {
	case len([]rune(req.Instructions)) > maxInstruction:
		return prepared{}, apperr.ErrValidation.WithDetail("field", "instructions")
	case len([]rune(req.Change)) > maxInstruction:
		return prepared{}, apperr.ErrValidation.WithDetail("field", "change")
	case len([]rune(req.Draft)) > maxDraft:
		return prepared{}, apperr.ErrValidation.WithDetail("field", "draft")
	case change && strings.TrimSpace(req.Draft) == "":
		return prepared{}, apperr.ErrValidation.WithDetail("field", "draft")
	case change && req.Change == "":
		return prepared{}, apperr.ErrValidation.WithDetail("field", "change")
	case !change && req.TemplateID == nil:
		return prepared{}, apperr.ErrValidation.WithDetail("field", "template_id")
	}
	system := Rules
	if req.TemplateID != nil {
		t, err := g.svc.template(ctx, q, *req.TemplateID)
		if err != nil {
			return prepared{}, err
		}
		if t.ArchivedAt != nil && !change {
			return prepared{}, errTemplateArchived // RF-25
		}
		system += "\n\n## Instrucciones de este tipo de informe: " + t.Name + "\n\n" + t.Prompt
	}

	// The tree: the project and every descendant the person can see (RF-45, RF-47).
	rows, err := q.ProjectSubtree(ctx, &projectID)
	if err != nil {
		return prepared{}, fmt.Errorf("subárbol: %w", err)
	}
	names := map[string]string{project.ID.String(): project.Name}
	for _, r := range rows {
		if r.IsTemplate && !p.Has(authz.TemplatesUse) {
			continue
		}
		names[r.ID.String()] = r.Name
	}
	scope := mcp.ReportScope{Root: project.ID.String(), InTree: func(id string) bool { _, ok := names[id]; return ok }}

	today := g.now().Format("2006-01-02")
	var prompt string
	if change {
		prompt = fmt.Sprintf("Proyecto del informe: %s (id %s). Fecha de hoy: %s.\n\nEste es el informe actual:\n\n<informe>\n%s\n</informe>\n\n"+
			"Cambio que pide la persona: %s\n\nAplica el cambio, consultando datos si hace falta, y devuelve el informe completo.",
			project.Name, project.ID, today, req.Draft, req.Change)
	} else {
		prompt = fmt.Sprintf("Proyecto del informe: %s (id %s). Fecha de hoy: %s.\n\nEscribe el informe.", project.Name, project.ID, today)
		if req.Instructions != "" {
			prompt += "\n\nInstrucciones adicionales de la persona: " + req.Instructions
		}
	}
	return prepared{project: project.Name, system: system, prompt: prompt, scope: scope, names: names}, nil
}

// Generate streams a new report or a change to a draft (RF-36 to RF-61, RF-71 to RF-77).
func (g *Generator) Generate(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, req generateRequest) error {
	p := authz.FromContext(r.Context())
	prep, err := g.prepare(r.Context(), p, projectID, req)
	if err != nil {
		return err
	}
	if !g.acquire(p.UserID) {
		return errBusy
	}
	defer g.release(p.UserID)

	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return fmt.Errorf("token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(secret)
	sess := &session{p: p, scope: prep.scope, names: prep.names, events: make(chan Event, 64)}
	key := hashToken(token)
	g.mu.Lock()
	g.sessions[key] = sess
	g.mu.Unlock()
	defer func() { // the token is worthless once the generation ends
		g.mu.Lock()
		delete(g.sessions, key)
		g.mu.Unlock()
	}()

	ai := g.svc.ai()
	body, _ := json.Marshal(map[string]any{
		"llm":             map[string]string{"base_url": llm.Base(ai.BaseURL), "api_key": ai.APIKey, "model": ai.Model},
		"system_prompt":   prep.system,
		"prompt":          prep.prompt,
		"tools":           g.tools.ReportTools(),
		"tools_url":       g.callbackURL() + "/api/reports/agent/tools",
		"token":           token,
		"timeout_seconds": int(g.maxTime.Seconds()),
	})

	ctx, cancel := context.WithTimeout(r.Context(), g.maxTime)
	defer cancel()
	agentReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.svc.agent()+"/generate", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("petición al agente: %w", err)
	}
	agentReq.Header.Set("Content-Type", "application/json")

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	out := newStream(w)

	res, err := g.client.Do(agentReq)
	if err != nil {
		g.logger.Warn("agente de informes no disponible", "error", err)
		out.send(Event{Type: "error", Code: g.timeoutOr(ctx, r, "agent_unavailable")})
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		g.logger.Warn("agente de informes", "status", res.StatusCode)
		out.send(Event{Type: "error", Code: "agent_unavailable"})
		return nil
	}

	lines := make(chan []byte)
	readErr := make(chan error, 1)
	go func() {
		sc := bufio.NewScanner(res.Body)
		sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for sc.Scan() {
			line := append([]byte(nil), sc.Bytes()...)
			select {
			case lines <- line:
			case <-ctx.Done():
				return
			}
		}
		readErr <- sc.Err()
	}()

	ping := time.NewTicker(PingEvery)
	defer ping.Stop()
	var text strings.Builder
	for {
		select {
		case <-ctx.Done():
			out.send(Event{Type: "error", Code: g.timeoutOr(ctx, r, "")})
			return nil
		case ev := <-sess.events:
			out.send(ev)
		case <-ping.C:
			out.send(Event{Type: "ping"})
		case err := <-readErr:
			if err != nil {
				out.send(Event{Type: "error", Code: g.timeoutOr(ctx, r, "agent_unavailable")})
			} else {
				out.send(Event{Type: "error", Code: "agent_unavailable"}) // ended without done
			}
			return nil
		case line := <-lines:
			var ev Event
			if len(bytes.TrimSpace(line)) == 0 || json.Unmarshal(line, &ev) != nil {
				continue
			}
			switch ev.Type {
			case "todos":
				out.send(Event{Type: "todos", Todos: ev.Todos})
			case "text":
				text.WriteString(ev.Delta)
				out.send(Event{Type: "text", Delta: ev.Delta})
			case "reset":
				text.Reset()
				out.send(Event{Type: "reset"})
			case "error":
				drain(sess, out)
				out.send(Event{Type: "error", Code: aiCode(ev.Code)})
				return nil
			case "done":
				drain(sess, out)
				if strings.TrimSpace(text.String()) == "" {
					out.send(Event{Type: "error", Code: "ai_empty"}) // RF-76
				} else {
					out.send(Event{Type: "done"})
				}
				return nil
			}
		}
	}
}

// timeoutOr names why the context ended: the 10 minutes (RF-72), or the
// person leaving, when nobody reads the stream any more.
func (g *Generator) timeoutOr(ctx context.Context, r *http.Request, otherwise string) string {
	if r.Context().Err() == nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "ai_too_long"
	}
	if otherwise == "" {
		return "cancelled"
	}
	return otherwise
}

// aiCode keeps only the codes the interface knows (RF-73).
func aiCode(code string) string {
	switch code {
	case "ai_unreachable", "ai_auth", "ai_model", "ai_quota", "ai_context_length", "ai_timeout", "ai_tools", "ai_rejected":
		return code
	}
	return "ai_error"
}

// drain sends the progress still queued, so it comes before the end.
func drain(sess *session, out *stream) {
	for {
		select {
		case ev := <-sess.events:
			out.send(ev)
		default:
			return
		}
	}
}

// stream writes NDJSON lines and flushes each one.
type stream struct {
	w http.ResponseWriter
	f http.Flusher
}

func newStream(w http.ResponseWriter) *stream {
	f, _ := w.(http.Flusher)
	return &stream{w: w, f: f}
}

func (s *stream) send(ev Event) {
	b, _ := json.Marshal(ev)
	_, _ = s.w.Write(append(b, '\n'))
	if s.f != nil {
		s.f.Flush()
	}
}

// ServeTool answers a query of the agent (RF-45 to RF-52). Its only
// credential is the token of a running generation.
func (g *Generator) ServeTool(w http.ResponseWriter, r *http.Request, tool string) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	g.mu.Lock()
	sess := g.sessions[hashToken(strings.TrimSpace(token))]
	g.mu.Unlock()
	if !ok || sess == nil {
		apperr.Write(w, g.logger, apperr.ErrUnauthenticated)
		return
	}
	var args map[string]any
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&args); err != nil {
		args = map[string]any{}
	}
	answer := func(text string) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"content": text})
	}
	sess.mu.Lock()
	sess.queries++
	n := sess.queries
	sess.mu.Unlock()
	if n > MaxQueries {
		answer(fmt.Sprintf("Has alcanzado el límite de %d consultas. No consultes nada más: termina el informe con lo que ya tienes.", MaxQueries))
		return
	}
	text, pid, err := g.tools.RunReportTool(r.Context(), sess.p, sess.scope, tool, args)
	if errors.Is(err, mcp.ErrUnknownReportTool) {
		answer("Error: no existe la herramienta " + tool + ".")
		return
	}
	if err != nil {
		g.logger.Error("consulta del agente de informes", "tool", tool, "error", err)
		answer("Error interno de Kairos.")
		return
	}
	select { // the progress is best effort; never block the agent
	case sess.events <- Event{Type: "tool", Tool: tool, Project: sess.names[pid]}:
	default:
	}
	answer(text)
}
