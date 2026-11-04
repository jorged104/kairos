// Package gamification turns closed work items into XP and coins through an
// immutable ledger, derives levels from XP and ranks the members of a team
// (spec 014). Points are only ever written here, never by the frontend.
package gamification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/db"
	"openscum/internal/db/sqlc"
	"openscum/internal/events"
)

// ConsumerName is the cursor of the consumer; the migration starts it after
// the last event, so the work closed before is not paid (RF-53).
const ConsumerName = "gamification"

const (
	reasonClose  = "close"
	reasonReopen = "reopen"
	reasonAdjust = "adjust"
)

type closePayload struct {
	AssigneeIDs []uuid.UUID     `json:"assignee_ids"`
	SizeID      json.RawMessage `json:"size_id"`
}

// Consumer pays the assignees of a closed work item and takes it back when it
// reopens (RF-6 to RF-17). Every event is one transaction.
func Consumer(store *db.Store) events.Consumer {
	return func(ctx context.Context, e events.Event) error {
		if e.SubjectType != "work_item" {
			return nil
		}
		switch e.Type {
		case events.WorkItemClosed:
			return store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error { return onClosed(ctx, q, e) })
		case events.WorkItemReopened:
			return store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error { return onReopened(ctx, q, e) })
		}
		return nil
	}
}

func onClosed(ctx context.Context, q *sqlc.Queries, e events.Event) error {
	var p closePayload
	if err := json.Unmarshal(e.Payload, &p); err != nil {
		return fmt.Errorf("leer evento: %w", err)
	}
	if len(p.AssigneeIDs) == 0 {
		return nil // RF-8
	}
	wi, err := q.PointsWorkItem(ctx, e.SubjectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // removed meanwhile
	}
	if err != nil {
		return fmt.Errorf("leer work item: %w", err)
	}
	if wi.IsTemplate {
		return nil // RF-9
	}
	// The size of the close travels in the event; an event of a previous
	// version of the API has none, and the current size stands in.
	size := wi.SizeID
	if len(p.SizeID) > 0 {
		size = nil
		if string(p.SizeID) != "null" {
			var id uuid.UUID
			if err := json.Unmarshal(p.SizeID, &id); err != nil {
				return fmt.Errorf("leer talla: %w", err)
			}
			size = &id
		}
	}
	value, err := q.GetPointValue(ctx, size)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // a size without a row is worth nothing (RF-25)
	}
	if err != nil {
		return fmt.Errorf("leer tabla de puntos: %w", err)
	}
	if value.Xp == 0 && value.Coins == 0 {
		return nil // RF-10
	}
	eventID, itemID := e.EventID, e.SubjectID
	seen := map[uuid.UUID]bool{}
	for _, u := range p.AssigneeIDs {
		if seen[u] {
			continue
		}
		seen[u] = true
		if _, err := q.InsertPointMovement(ctx, sqlc.InsertPointMovementParams{
			UserID: u, Xp: value.Xp, Coins: value.Coins, Reason: reasonClose, WorkItemID: &itemID,
			EventID: &eventID, ActorID: e.ActorID, CreatedAt: e.OccurredAt,
		}); err != nil {
			return fmt.Errorf("registrar puntos: %w", err)
		}
	}
	return nil
}

func onReopened(ctx context.Context, q *sqlc.Queries, e events.Event) error {
	itemID := e.SubjectID
	closeID, err := q.LastCloseEvent(ctx, &itemID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // RF-16, RF-54
	}
	if err != nil {
		return fmt.Errorf("leer último cierre: %w", err)
	}
	// A close already taken back belongs to an earlier cycle, so the last
	// close paid nobody (RF-16); it is also the case of a repeated delivery of
	// this very event (RF-12).
	n, err := q.CountReversals(ctx, &closeID)
	if err != nil {
		return fmt.Errorf("leer compensaciones: %w", err)
	}
	if n > 0 {
		return nil
	}
	paid, err := q.ListEventMovements(ctx, &closeID)
	if err != nil {
		return fmt.Errorf("leer cierre: %w", err)
	}
	eventID := e.EventID
	for _, m := range paid {
		if _, err := q.InsertPointMovement(ctx, sqlc.InsertPointMovementParams{
			UserID: m.UserID, Xp: -m.Xp, Coins: -m.Coins, Reason: reasonReopen, WorkItemID: &itemID,
			EventID: &eventID, ReversesEventID: &closeID, ActorID: e.ActorID, CreatedAt: e.OccurredAt,
		}); err != nil {
			return fmt.Errorf("registrar compensación: %w", err)
		}
	}
	return nil
}
