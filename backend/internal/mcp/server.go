package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"openscum/internal/audit"
	"openscum/internal/authz"
)

// The Streamable HTTP transport in its smallest form: one JSON-RPC message
// per POST, answered with application/json; no event stream, no sessions.

// Versions of the protocol the server speaks, newest first.
var Versions = []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}

const (
	// RateLimit is how many requests a connection may make per minute (RF-47).
	RateLimit = 120
	maxBody   = 1 << 20
)

// JSON-RPC error codes; the server's own are in the range the spec leaves free.
const (
	codeParse          = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeUnavailable    = -32001
)

var (
	msgDisabled = "El acceso por IA está desactivado en Kairos. Pide a la persona que administra el sistema que lo active."
	msgUnknown  = "Esta URL de conexión no existe, caducó o fue revocada. Crea una conexión nueva en Mi cuenta de Kairos."
	msgInactive = "La cuenta dueña de esta conexión no está activa en Kairos."
	msgLimit    = "Demasiadas peticiones con esta conexión. Reintenta en unos segundos."
)

// PrincipalLoader loads who owns a connection, as the session middleware does.
type PrincipalLoader interface {
	Load(ctx context.Context, id uuid.UUID) (*authz.Principal, error)
}

// Server answers the MCP requests of every connection.
type Server struct {
	conns   *Connections
	loader  PrincipalLoader
	enabled func() bool
	logger  *slog.Logger
	tools   []tool
	limiter *limiter

	// api serves the internal requests; it is the app's router, set once
	// built because the server is mounted on it.
	api http.Handler
}

func NewServer(conns *Connections, loader PrincipalLoader, enabled func() bool, logger *slog.Logger) *Server {
	return &Server{conns: conns, loader: loader, enabled: enabled, logger: logger, tools: append(tools(), meetingTools()...), limiter: newLimiter(RateLimit, time.Minute)}
}

// SetAPI sets the router the tools call.
func (s *Server) SetAPI(api http.Handler) { s.api = api }

// SetRateLimit changes the limit, for the tests.
func (s *Server) SetRateLimit(n int) { s.limiter = newLimiter(n, time.Minute) }

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

func writeRPC(w http.ResponseWriter, status int, id json.RawMessage, result any, e *rpcError) {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(rpcResponse{JSONRPC: "2.0", ID: id, Result: result, Error: e})
}

// ServeHTTP is mounted at Path + {secret}.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		// No server-initiated stream: the client sends POSTs only.
		w.Header().Set("Allow", http.MethodPost)
		writeRPC(w, http.StatusMethodNotAllowed, nil, nil, &rpcError{Code: codeInvalidRequest, Message: "Solo se admite POST."})
		return
	}
	if !s.enabled() {
		writeRPC(w, http.StatusForbidden, nil, nil, &rpcError{Code: codeUnavailable, Message: msgDisabled})
		return
	}
	ctx := r.Context()
	conn, err := s.conns.authenticate(ctx, chi.URLParam(r, "secret"))
	if errors.Is(err, errUnknownSecret) {
		writeRPC(w, http.StatusNotFound, nil, nil, &rpcError{Code: codeUnavailable, Message: msgUnknown})
		return
	}
	if err != nil {
		s.logger.Error("autenticar conexión MCP", slog.Any("error", err))
		writeRPC(w, http.StatusInternalServerError, nil, nil, &rpcError{Code: codeUnavailable, Message: "Error interno"})
		return
	}
	if wait, ok := s.limiter.allow(conn.ID); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		writeRPC(w, http.StatusTooManyRequests, nil, nil, &rpcError{Code: codeUnavailable, Message: msgLimit})
		return
	}
	p, err := s.loader.Load(ctx, conn.UserID)
	if err != nil || p.Status != "active" {
		writeRPC(w, http.StatusForbidden, nil, nil, &rpcError{Code: codeUnavailable, Message: msgInactive})
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	var req rpcRequest
	if err != nil || json.Unmarshal(body, &req) != nil || req.JSONRPC != "2.0" || req.Method == "" {
		writeRPC(w, http.StatusBadRequest, nil, nil, &rpcError{Code: codeParse, Message: "Mensaje JSON-RPC no válido (no se admiten lotes)."})
		return
	}

	client := ""
	if req.Method == "initialize" {
		var init struct {
			ClientInfo struct {
				Name string `json:"name"`
			} `json:"clientInfo"`
		}
		_ = json.Unmarshal(req.Params, &init)
		client = truncate(init.ClientInfo.Name, 100)
	}
	s.conns.touch(context.WithoutCancel(ctx), conn.ID, client)

	if len(req.ID) == 0 || string(req.ID) == "null" {
		// A notification (initialized, cancelled…): nothing to answer.
		w.WriteHeader(http.StatusAccepted)
		return
	}

	via := audit.Via{ConnectionID: conn.ID, Name: conn.Name}
	if client != "" {
		via.Client = client
	} else if conn.LastClient != nil {
		via.Client = *conn.LastClient
	}
	result, rerr := s.handle(audit.WithVia(ctx, via), p, req)
	writeRPC(w, http.StatusOK, req.ID, result, rerr)
}

func (s *Server) handle(ctx context.Context, p *authz.Principal, req rpcRequest) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		var params struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &params)
		version := Versions[0]
		if slices.Contains(Versions, params.ProtocolVersion) {
			version = params.ProtocolVersion
		}
		return map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "kairos", "title": "Kairos", "version": "1.0.0"},
			"instructions":    instructions,
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		list := make([]map[string]any, 0, len(s.tools))
		for _, t := range s.tools {
			if t.offered(p) {
				list = append(list, t.definition())
			}
		}
		return map[string]any{"tools": list}, nil
	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, &rpcError{Code: codeInvalidParams, Message: "Parámetros no válidos."}
		}
		for _, t := range s.tools {
			if t.name == params.Name && t.offered(p) {
				return s.call(ctx, p, t, params.Arguments), nil
			}
		}
		return nil, &rpcError{Code: codeInvalidParams, Message: "Herramienta desconocida: " + params.Name}
	}
	return nil, &rpcError{Code: codeMethodNotFound, Message: "Método no admitido: " + req.Method}
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// limiter counts the requests of each connection in fixed windows (RF-47).
// It lives in memory, like the login limiter: enough for one API instance.
type limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	now    func() time.Time
	counts map[uuid.UUID]*windowCount
}

type windowCount struct {
	start time.Time
	n     int
}

func newLimiter(max int, window time.Duration) *limiter {
	return &limiter{max: max, window: window, now: time.Now, counts: map[uuid.UUID]*windowCount{}}
}

func (l *limiter) allow(id uuid.UUID) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	c := l.counts[id]
	if c == nil || now.Sub(c.start) >= l.window {
		c = &windowCount{start: now}
		l.counts[id] = c
	}
	if c.n >= l.max {
		return c.start.Add(l.window).Sub(now), false
	}
	c.n++
	return 0, true
}

const instructions = `Kairos es una herramienta de gestión de proyectos. Estas herramientas actúan como la persona dueña de la conexión, con sus mismos permisos.
- Para encontrar algo por nombre usa search; cada resultado trae el id que piden las demás herramientas y el enlace a Kairos.
- Antes de crear o editar tareas, consulta project_catalog para conocer los tipos, estados, prioridades, tallas, etiquetas y personas del proyecto.
- Las descripciones, comentarios y páginas se leen y escriben en Markdown.
- No se puede borrar, archivar ni cancelar nada desde aquí: si la persona lo pide, indícale que lo haga en Kairos.
- Si una edición devuelve version_conflict, vuelve a leer el elemento antes de reintentar.`
