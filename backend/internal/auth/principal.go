package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/authz"
	"openscum/internal/db"
)

// PrincipalTTL bounds how long a role, scope or status change may take to
// apply to requests served by another instance (RF-44 allows 120 s).
const PrincipalTTL = 60 * time.Second

// ErrUserNotFound means the session points at a user that does not exist.
var ErrUserNotFound = errors.New("usuario no encontrado")

// PrincipalLoader reads roles, permissions, scope and status from the
// database (RF-43), with a short per-user cache.
type PrincipalLoader struct {
	store *db.Store
	ttl   time.Duration
	now   func() time.Time

	mu    sync.Mutex
	cache map[uuid.UUID]cachedPrincipal
}

type cachedPrincipal struct {
	p       *authz.Principal
	expires time.Time
}

func NewPrincipalLoader(store *db.Store, ttl time.Duration) *PrincipalLoader {
	return &PrincipalLoader{store: store, ttl: ttl, now: time.Now, cache: map[uuid.UUID]cachedPrincipal{}}
}

func (l *PrincipalLoader) Load(ctx context.Context, id uuid.UUID) (*authz.Principal, error) {
	l.mu.Lock()
	c, ok := l.cache[id]
	l.mu.Unlock()
	if ok && l.now().Before(c.expires) {
		return c.p, nil
	}

	p, err := l.fetch(ctx, id)
	if err != nil {
		return nil, err
	}
	l.mu.Lock()
	l.cache[id] = cachedPrincipal{p: p, expires: l.now().Add(l.ttl)}
	l.mu.Unlock()
	return p, nil
}

func (l *PrincipalLoader) fetch(ctx context.Context, id uuid.UUID) (*authz.Principal, error) {
	u, err := l.store.GetUser(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("cargar usuario: %w", err)
	}
	rows, err := l.store.ListUserRoles(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("cargar roles: %w", err)
	}
	roles := make([]authz.Role, 0, len(rows))
	for _, r := range rows {
		roles = append(roles, authz.Role(r.Role))
	}
	scope, err := l.store.LeaderScopeTeamIDs(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("cargar ámbito: %w", err)
	}
	participant, err := l.store.ParticipantTeamIDs(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("cargar equipos: %w", err)
	}
	return authz.NewPrincipal(u.ID, u.Status, roles, scope).WithParticipantTeams(participant), nil
}

// Invalidate drops one user's cached principal after a change made here.
func (l *PrincipalLoader) Invalidate(id uuid.UUID) {
	l.mu.Lock()
	delete(l.cache, id)
	l.mu.Unlock()
}

// InvalidateAll drops every cached principal, e.g. after a team hierarchy change.
func (l *PrincipalLoader) InvalidateAll() {
	l.mu.Lock()
	l.cache = map[uuid.UUID]cachedPrincipal{}
	l.mu.Unlock()
}
