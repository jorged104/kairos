// Package events records domain events in the same transaction as the change
// that causes them (outbox) and delivers them to in-process consumers at
// least once and in order (RF-116 to RF-122).
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"openscum/internal/db/sqlc"
)

const (
	WorkItemClosed     = "work_item.closed"
	WorkItemReopened   = "work_item.reopened"
	MilestoneCompleted = "milestone.completed"
	MilestoneReopened  = "milestone.reopened"
)

// Event is a recorded domain event. EventID is unique and lets consumers
// ignore a repeated delivery.
type Event struct {
	ID          int64           `json:"id"`
	EventID     uuid.UUID       `json:"event_id"`
	Type        string          `json:"type"`
	OccurredAt  time.Time       `json:"occurred_at"`
	ActorID     *uuid.UUID      `json:"actor_id"`
	ProjectID   *uuid.UUID      `json:"project_id"`
	SubjectType string          `json:"subject_type"`
	SubjectID   uuid.UUID       `json:"subject_id"`
	Payload     json.RawMessage `json:"payload"`
}

// New describes an event to emit; Payload is marshalled to JSON.
type New struct {
	Type        string
	ActorID     *uuid.UUID
	ProjectID   uuid.UUID
	SubjectType string
	SubjectID   uuid.UUID
	Payload     any
}

// Emit writes the event with the caller's queries: if the change rolls back,
// the event is gone too (RF-120). It takes the outbox lock first, so ids
// commit in order and a reader by cursor never skips a slower transaction.
func Emit(ctx context.Context, q *sqlc.Queries, e New) error {
	payload := json.RawMessage("{}")
	if e.Payload != nil {
		b, err := json.Marshal(e.Payload)
		if err != nil {
			return fmt.Errorf("serializar evento: %w", err)
		}
		payload = b
	}
	if err := q.LockDomainEvents(ctx); err != nil {
		return fmt.Errorf("bloquear eventos: %w", err)
	}
	project := e.ProjectID
	if _, err := q.InsertDomainEvent(ctx, sqlc.InsertDomainEventParams{
		Type: e.Type, ActorID: e.ActorID, ProjectID: &project,
		SubjectType: e.SubjectType, SubjectID: e.SubjectID, Payload: payload,
	}); err != nil {
		return fmt.Errorf("registrar evento: %w", err)
	}
	return nil
}

func fromRow(r sqlc.DomainEvent) Event {
	return Event{
		ID: r.ID, EventID: r.EventID, Type: r.Type, OccurredAt: r.OccurredAt, ActorID: r.ActorID,
		ProjectID: r.ProjectID, SubjectType: r.SubjectType, SubjectID: r.SubjectID, Payload: r.Payload,
	}
}
