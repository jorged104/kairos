// Package audit records who changed what and when (RF-117 to RF-120).
package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"openscum/internal/authz"
	"openscum/internal/db/sqlc"
)

const (
	ActorUser   = "user"
	ActorIdP    = "idp"
	ActorSystem = "system"
)

// Actor is who made a change.
type Actor struct {
	ID   *uuid.UUID
	Kind string
}

// ByUser is the actor for a change made by a signed-in user.
func ByUser(p *authz.Principal) Actor {
	id := p.UserID
	return Actor{ID: &id, Kind: ActorUser}
}

// IdP is the actor for changes made by the login synchronization.
var IdP = Actor{Kind: ActorIdP}

// System is the actor for changes with no human behind them (bootstrap, self registration).
var System = Actor{Kind: ActorSystem}

// Entry is one audited change. Before and After are marshalled to JSON.
type Entry struct {
	Actor      Actor
	Action     string
	TargetType string
	TargetID   string
	Before     any
	After      any
}

// Record writes the entry with the caller's queries, so it is part of the
// same transaction as the change: a change without its audit never commits.
func Record(ctx context.Context, q *sqlc.Queries, e Entry) error {
	before, err := marshal(e.Before)
	if err != nil {
		return err
	}
	after, err := marshal(e.After)
	if err != nil {
		return err
	}
	if err := q.InsertAudit(ctx, sqlc.InsertAuditParams{
		ActorID: e.Actor.ID, ActorKind: e.Actor.Kind, Action: e.Action,
		TargetType: e.TargetType, TargetID: e.TargetID, Before: before, After: after,
	}); err != nil {
		return fmt.Errorf("registrar auditoría: %w", err)
	}
	return nil
}

func marshal(v any) (json.RawMessage, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("serializar auditoría: %w", err)
	}
	return b, nil
}
