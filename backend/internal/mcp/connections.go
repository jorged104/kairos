// Package mcp lets AI clients work in Kairos through the Model Context
// Protocol (spec 016). A person creates a connection and pastes its secret
// URL in the client; every tool runs as that person, through the same API
// routes the interface uses.
package mcp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/auth"
	"openscum/internal/authz"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
)

const (
	// Path is where the connections are served; the secret follows it.
	Path = auth.MCPPath
	// MaxConnections a person may have alive at once (RF-14).
	MaxConnections = 10
	// touchEvery bounds how often the last use is written.
	touchEvery = time.Minute
)

// Days are the expirations a person may choose (RF-9).
var Days = []int{30, 90, 365}

var (
	errDisabled        = apperr.New(http.StatusConflict, "mcp_disabled", "El acceso por IA está desactivado")
	errTooMany         = apperr.New(http.StatusConflict, "mcp_too_many_connections", "Ya tienes el máximo de conexiones vigentes")
	errUnknownSecret   = errors.New("conexión inexistente, caducada o revocada")
	errConnectionOwner = apperr.ErrNotFound
)

// Connection is what the lists show of a connection, never its secret (RF-19, RF-23).
type Connection struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	LastClient *string    `json:"last_client"`
	User       *Owner     `json:"user,omitempty"`
}

type Owner struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Email string    `json:"email"`
}

// Mine is the connections card of the account page.
type Mine struct {
	Enabled bool         `json:"enabled"`
	Max     int          `json:"max"`
	Items   []Connection `json:"items"`
}

// Created carries the URL, shown this once (RF-11, RF-12).
type Created struct {
	Connection
	URL string `json:"url"`
}

type CreateRequest struct {
	Name string `json:"name" validate:"required,max=100"`
	Days int    `json:"days" validate:"oneof=30 90 365"`
}

func (r *CreateRequest) Normalize() { r.Name = strings.TrimSpace(r.Name) }

// Connections creates, lists, revokes and authenticates connections.
type Connections struct {
	store     *db.Store
	enabled   func() bool
	publicURL func() string
	now       func() time.Time

	mu      sync.Mutex
	touched map[uuid.UUID]time.Time
}

func NewConnections(store *db.Store, enabled func() bool, publicURL func() string) *Connections {
	return &Connections{store: store, enabled: enabled, publicURL: publicURL, now: time.Now, touched: map[uuid.UUID]time.Time{}}
}

// URL is the address of a secret.
func (c *Connections) URL(secret string) string {
	return strings.TrimRight(c.publicURL(), "/") + Path + secret
}

func hash(secret string) []byte {
	h := sha256.Sum256([]byte(secret))
	return h[:]
}

func view(r sqlc.McpConnection) Connection {
	return Connection{ID: r.ID, Name: r.Name, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, LastUsedAt: r.LastUsedAt, LastClient: r.LastClient}
}

func (c *Connections) list(ctx context.Context, user *uuid.UUID) ([]Connection, error) {
	rows, err := c.store.ListMCPConnections(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("listar conexiones: %w", err)
	}
	out := make([]Connection, 0, len(rows))
	for _, r := range rows {
		v := Connection{ID: r.ID, Name: r.Name, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, LastUsedAt: r.LastUsedAt, LastClient: r.LastClient}
		if user == nil {
			v.User = &Owner{ID: r.UserID, Name: r.UserName, Email: r.UserEmail}
		}
		out = append(out, v)
	}
	return out, nil
}

// Mine lists the live connections of p (RF-19).
func (c *Connections) Mine(ctx context.Context, p *authz.Principal) (Mine, error) {
	items, err := c.list(ctx, &p.UserID)
	return Mine{Enabled: c.enabled(), Max: MaxConnections, Items: items}, err
}

// All lists everybody's live connections (RF-23).
func (c *Connections) All(ctx context.Context, p *authz.Principal) ([]Connection, error) {
	if !p.Has(authz.SystemManage) {
		return nil, apperr.ErrForbidden
	}
	return c.list(ctx, nil)
}

// Create makes a connection for p with a new secret (RF-8 to RF-14).
func (c *Connections) Create(ctx context.Context, p *authz.Principal, req CreateRequest) (Created, error) {
	if !c.enabled() {
		return Created{}, errDisabled
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return Created{}, fmt.Errorf("generar secreto: %w", err)
	}
	secret := base64.RawURLEncoding.EncodeToString(buf)
	var row sqlc.McpConnection
	err := c.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		// One person's creations are serialized by the lock of their user row.
		if _, err := q.GetUserForUpdate(ctx, p.UserID); err != nil {
			return fmt.Errorf("bloquear usuario: %w", err)
		}
		n, err := q.CountLiveMCPConnections(ctx, p.UserID)
		if err != nil {
			return fmt.Errorf("contar conexiones: %w", err)
		}
		if n >= MaxConnections {
			return errTooMany
		}
		row, err = q.CreateMCPConnection(ctx, sqlc.CreateMCPConnectionParams{
			UserID: p.UserID, Name: req.Name, SecretHash: hash(secret),
			ExpiresAt: c.now().Add(time.Duration(req.Days) * 24 * time.Hour),
		})
		if err != nil {
			return fmt.Errorf("crear conexión: %w", err)
		}
		return audit.Record(ctx, q, audit.Entry{
			Actor: audit.ByUser(p), Action: "mcp.connection_created", TargetType: "mcp_connection", TargetID: row.ID.String(),
			After: map[string]any{"name": row.Name, "user_id": row.UserID, "expires_at": row.ExpiresAt},
		})
	})
	if err != nil {
		return Created{}, err
	}
	return Created{Connection: view(row), URL: c.URL(secret)}, nil
}

// Revoke cuts a connection of p, or of anybody for a system administrator
// (RF-20, RF-24, RF-25). others is false on the account page.
func (c *Connections) Revoke(ctx context.Context, p *authz.Principal, id uuid.UUID, others bool) error {
	if others && !p.Has(authz.SystemManage) {
		return apperr.ErrForbidden
	}
	return c.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		row, err := q.GetMCPConnectionForUpdate(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && (row.RevokedAt != nil || (!others && row.UserID != p.UserID))) {
			return errConnectionOwner
		}
		if err != nil {
			return fmt.Errorf("leer conexión: %w", err)
		}
		if err := q.RevokeMCPConnection(ctx, sqlc.RevokeMCPConnectionParams{ID: id, RevokedBy: &p.UserID}); err != nil {
			return fmt.Errorf("revocar conexión: %w", err)
		}
		return audit.Record(ctx, q, audit.Entry{
			Actor: audit.ByUser(p), Action: "mcp.connection_revoked", TargetType: "mcp_connection", TargetID: id.String(),
			Before: map[string]any{"name": row.Name, "user_id": row.UserID}, After: map[string]any{"revoked_by": p.UserID},
		})
	})
}

// authenticate finds the live connection of a secret (RF-15, RF-20 to RF-22).
func (c *Connections) authenticate(ctx context.Context, secret string) (sqlc.McpConnection, error) {
	if secret == "" || len(secret) > 100 {
		return sqlc.McpConnection{}, errUnknownSecret
	}
	row, err := c.store.MCPConnectionBySecret(ctx, hash(secret))
	if errors.Is(err, pgx.ErrNoRows) {
		return row, errUnknownSecret
	}
	if err != nil {
		return row, fmt.Errorf("leer conexión: %w", err)
	}
	return row, nil
}

// touch writes the last use, at most once a minute per connection, and the
// client's name when it says it (RF-19).
func (c *Connections) touch(ctx context.Context, id uuid.UUID, client string) {
	c.mu.Lock()
	now := c.now()
	due := now.Sub(c.touched[id]) >= touchEvery || client != ""
	if due {
		c.touched[id] = now
	}
	c.mu.Unlock()
	if !due {
		return
	}
	var name *string
	if client != "" {
		name = &client
	}
	_ = c.store.TouchMCPConnection(ctx, sqlc.TouchMCPConnectionParams{ID: id, Client: name})
}
