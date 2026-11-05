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

	"openscum/internal/audit"
	"openscum/internal/db/sqlc"
	"openscum/internal/richtext"
)

const (
	WorkItemClosed     = "work_item.closed"
	WorkItemReopened   = "work_item.reopened"
	MilestoneCompleted = "milestone.completed"
	MilestoneReopened  = "milestone.reopened"

	// Spec 017: what the notifications tell.
	CommentCreated        = "comment.created"
	CommentThreadResolved = "comment.thread_resolved"
	CommentThreadReopened = "comment.thread_reopened"
	WorkItemStatusChanged = "work_item.status_changed"
	MentionAdded          = "mention.added"
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
	// A change made through an AI connection carries it, so what it causes
	// later can say «vía IA» too (spec 016 RF-44, spec 017 RF-18).
	if via := audit.ViaOf(ctx); via != nil {
		var fields map[string]json.RawMessage
		if json.Unmarshal(payload, &fields) == nil {
			b, _ := json.Marshal(via)
			fields["via"] = b
			if payload, err := json.Marshal(fields); err == nil {
				return emit(ctx, q, e, payload)
			}
		}
	}
	return emit(ctx, q, e, payload)
}

func emit(ctx context.Context, q *sqlc.Queries, e New, payload json.RawMessage) error {
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

// EmitMentions records the users mentioned in a description that were not
// mentioned before it was saved (spec 017, RF-11). Nothing new, no event.
func EmitMentions(ctx context.Context, q *sqlc.Queries, actor uuid.UUID, projectID uuid.UUID, subjectType string, subjectID uuid.UUID, before, after json.RawMessage) error {
	had := map[uuid.UUID]bool{}
	for _, m := range richtext.Mentions(before) {
		if m.Kind == "user" {
			had[m.ID] = true
		}
	}
	var added []uuid.UUID
	for _, m := range richtext.Mentions(after) {
		if m.Kind == "user" && !had[m.ID] {
			had[m.ID] = true
			added = append(added, m.ID)
		}
	}
	if len(added) == 0 {
		return nil
	}
	return Emit(ctx, q, New{
		Type: MentionAdded, ActorID: &actor, ProjectID: projectID, SubjectType: subjectType, SubjectID: subjectID,
		Payload: map[string]any{"user_ids": added},
	})
}
