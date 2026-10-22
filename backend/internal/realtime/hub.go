// Package realtime holds the WebSocket hub. Connections are authenticated
// with the session cookie and closed when the user stops being active
// (RF-47 to RF-49). Domain events arrive with later specs.
package realtime

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"openscum/internal/apperr"
	"openscum/internal/authz"
)

// CheckInterval plus the principal cache TTL (60 s) keeps RF-49 under 120 s.
const CheckInterval = 30 * time.Second

type SessionParser interface {
	Parse(r *http.Request) (uuid.UUID, error)
}

type PrincipalLoader interface {
	Load(ctx context.Context, id uuid.UUID) (*authz.Principal, error)
}

type Hub struct {
	sessions SessionParser
	loader   PrincipalLoader
	logger   *slog.Logger
	upgrader websocket.Upgrader
	// Interval is how often connected users are re-checked; tests shorten it.
	Interval time.Duration

	mu    sync.Mutex
	conns map[uuid.UUID]map[*websocket.Conn]struct{}
}

func NewHub(sessions SessionParser, loader PrincipalLoader, publicURL string, logger *slog.Logger) *Hub {
	allowed, _ := url.Parse(publicURL)
	return &Hub{
		sessions: sessions, loader: loader, logger: logger, Interval: CheckInterval,
		conns: map[uuid.UUID]map[*websocket.Conn]struct{}{},
		upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
			o, err := url.Parse(r.Header.Get("Origin"))
			return err == nil && o.Scheme == allowed.Scheme && o.Host == allowed.Host
		}},
	}
}

// ServeWS upgrades the request only for an active user with a valid session (RF-48).
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	id, err := h.sessions.Parse(r)
	if err != nil {
		apperr.Write(w, h.logger, apperr.ErrUnauthenticated)
		return
	}
	p, err := h.loader.Load(r.Context(), id)
	if err != nil {
		apperr.Write(w, h.logger, apperr.ErrUnauthenticated)
		return
	}
	if p.Status != "active" {
		apperr.Write(w, h.logger, apperr.ErrAccountInactive)
		return
	}
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // the upgrader already answered
	}
	h.add(id, conn)
	defer h.remove(id, conn)
	for {
		if _, _, err := conn.NextReader(); err != nil {
			return
		}
	}
}

func (h *Hub) add(id uuid.UUID, c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conns[id] == nil {
		h.conns[id] = map[*websocket.Conn]struct{}{}
	}
	h.conns[id][c] = struct{}{}
}

func (h *Hub) remove(id uuid.UUID, c *websocket.Conn) {
	h.mu.Lock()
	delete(h.conns[id], c)
	if len(h.conns[id]) == 0 {
		delete(h.conns, id)
	}
	h.mu.Unlock()
	_ = c.Close()
}

// Run re-checks connected users until ctx ends, pinging the ones still
// active and closing the connections of the rest (RF-49).
func (h *Hub) Run(ctx context.Context) {
	t := time.NewTicker(h.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			h.check(ctx)
		}
	}
}

func (h *Hub) check(ctx context.Context) {
	h.mu.Lock()
	users := make(map[uuid.UUID][]*websocket.Conn, len(h.conns))
	for id, cs := range h.conns {
		for c := range cs {
			users[id] = append(users[id], c)
		}
	}
	h.mu.Unlock()

	deadline := time.Now().Add(5 * time.Second)
	for id, conns := range users {
		p, err := h.loader.Load(ctx, id)
		active := err == nil && p.Status == "active"
		for _, c := range conns {
			if active {
				_ = c.WriteControl(websocket.PingMessage, nil, deadline)
				continue
			}
			msg := websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "account_inactive")
			_ = c.WriteControl(websocket.CloseMessage, msg, deadline)
			_ = c.Close()
		}
	}
}
