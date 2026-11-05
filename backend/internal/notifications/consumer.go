// Package notifications tells people about comments, mentions and changes of
// the work items they follow (spec 017). The domain events become
// notifications in a consumer of the outbox, and each one reaches its
// recipient's open tabs through the WebSocket hub.
package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/authz"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
	"openscum/internal/events"
)

// ConsumerName is the cursor of the consumer; the migration starts it after
// the last event, so nothing that happened before is told.
const ConsumerName = "notifications"

// Kinds of the last event of a notification.
const (
	KindComment        = "comment"
	KindReply          = "reply"
	KindMention        = "mention"
	KindStatus         = "status"
	KindThreadResolved = "thread_resolved"
	KindThreadReopened = "thread_reopened"
)

// Sender delivers a message to every open tab of a user: the realtime hub.
type Sender interface {
	Send(userID uuid.UUID, v any)
}

// PrincipalLoader tells whether a recipient is active and may read the work.
type PrincipalLoader interface {
	Load(ctx context.Context, id uuid.UUID) (*authz.Principal, error)
}

type payload struct {
	CommentID    *uuid.UUID      `json:"comment_id"`
	ThreadID     *uuid.UUID      `json:"thread_id"`
	Reply        bool            `json:"reply"`
	Mentions     []uuid.UUID     `json:"mentions"`
	Excerpt      string          `json:"excerpt"`
	FromStatusID *uuid.UUID      `json:"from_status_id"`
	ToStatusID   *uuid.UUID      `json:"to_status_id"`
	UserIDs      []uuid.UUID     `json:"user_ids"`
	Via          json.RawMessage `json:"via"`
}

// recipients maps each person to whether one of their reasons is a mention (RF-17).
type recipients map[uuid.UUID]bool

func (r recipients) add(id uuid.UUID, mention bool) { r[id] = r[id] || mention }

// Consumer turns the events of spec 017 into notifications, one transaction
// per event, and then pushes them to the recipients.
func Consumer(store *db.Store, svc *Service, loader PrincipalLoader, logger *slog.Logger) events.Consumer {
	return func(ctx context.Context, e events.Event) error {
		if e.ActorID == nil || e.ProjectID == nil {
			return nil
		}
		var p payload
		if err := json.Unmarshal(e.Payload, &p); err != nil {
			return fmt.Errorf("leer evento: %w", err)
		}
		type created struct{ user, id uuid.UUID }
		var done []created
		err := store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
			to, kind, data, err := plan(ctx, q, e, p)
			if err != nil || kind == "" {
				return err
			}
			delete(to, *e.ActorID) // RF-15
			ids := make([]uuid.UUID, 0, len(to))
			for id := range to {
				ids = append(ids, id)
			}
			slices.SortFunc(ids, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
			for _, user := range ids {
				// Only active people who may read the work (RF-16); every
				// project but templates is readable with work:read.
				pr, err := loader.Load(ctx, user)
				if err != nil || pr.Status != "active" || !pr.Has(authz.WorkRead) {
					continue
				}
				// A repeated delivery of the event counts once.
				n, err := q.MarkNotified(ctx, sqlc.MarkNotifiedParams{EventID: e.EventID, UserID: user})
				if err != nil {
					return fmt.Errorf("marcar evento: %w", err)
				}
				if n == 0 {
					continue
				}
				id, err := q.UpsertNotification(ctx, sqlc.UpsertNotificationParams{
					UserID: user, SubjectType: e.SubjectType, SubjectID: e.SubjectID, ProjectID: *e.ProjectID,
					Actor: *e.ActorID, Mention: to[user], LastKind: kind, LastData: data, Via: p.Via, At: e.OccurredAt,
				})
				if err != nil {
					return fmt.Errorf("guardar notificación: %w", err)
				}
				done = append(done, created{user, id})
			}
			return nil
		})
		if err != nil {
			return err
		}
		// After the commit: what each recipient's tabs show (RF-33, RF-34).
		for _, c := range done {
			svc.push(ctx, c.user, &c.id)
		}
		return nil
	}
}

// plan says who hears about an event, as what, and what it shows (RF-7 to RF-14).
func plan(ctx context.Context, q *sqlc.Queries, e events.Event, p payload) (recipients, string, json.RawMessage, error) {
	to := recipients{}
	data := map[string]any{}
	var kind string
	follows := func() (followers, stopped map[uuid.UUID]bool, err error) {
		followers, stopped = map[uuid.UUID]bool{}, map[uuid.UUID]bool{}
		rows, err := q.WorkItemFollows(ctx, e.SubjectID)
		if err != nil {
			return nil, nil, fmt.Errorf("leer seguidores: %w", err)
		}
		for _, r := range rows {
			if r.Following {
				followers[r.UserID] = true
			} else {
				stopped[r.UserID] = true
			}
		}
		return followers, stopped, nil
	}
	participants := func(thread uuid.UUID, stopped map[uuid.UUID]bool) error {
		ids, err := q.ThreadParticipants(ctx, thread)
		if err != nil {
			return fmt.Errorf("leer participantes: %w", err)
		}
		for _, id := range ids {
			if !stopped[id] {
				to.add(id, false)
			}
		}
		return nil
	}

	switch e.Type {
	case events.CommentCreated:
		followers, stopped, err := follows()
		if err != nil {
			return nil, "", nil, err
		}
		for id := range followers {
			to.add(id, false) // RF-7, RF-8
		}
		kind = KindComment
		if p.Reply && p.ThreadID != nil {
			kind = KindReply
			if err := participants(*p.ThreadID, stopped); err != nil { // RF-9
				return nil, "", nil, err
			}
		}
		for _, id := range p.Mentions {
			to.add(id, true) // RF-10, RF-12
		}
		data["comment_id"], data["excerpt"] = p.CommentID, p.Excerpt
	case events.CommentThreadResolved, events.CommentThreadReopened:
		_, stopped, err := follows()
		if err != nil {
			return nil, "", nil, err
		}
		if p.ThreadID == nil {
			return nil, "", nil, nil
		}
		if err := participants(*p.ThreadID, stopped); err != nil { // RF-14
			return nil, "", nil, err
		}
		kind = KindThreadResolved
		if e.Type == events.CommentThreadReopened {
			kind = KindThreadReopened
		}
		data["comment_id"] = p.ThreadID
	case events.WorkItemStatusChanged:
		followers, _, err := follows()
		if err != nil {
			return nil, "", nil, err
		}
		for id := range followers {
			to.add(id, false) // RF-13
		}
		kind = KindStatus
		for key, id := range map[string]*uuid.UUID{"from": p.FromStatusID, "to": p.ToStatusID} {
			if id == nil {
				continue
			}
			s, err := q.GetStatus(ctx, *id)
			if err != nil {
				return nil, "", nil, fmt.Errorf("leer estado: %w", err)
			}
			data[key] = s.Name
		}
	case events.MentionAdded:
		for _, id := range p.UserIDs {
			to.add(id, true) // RF-11, RF-12
		}
		kind = KindMention
	default:
		return nil, "", nil, nil
	}
	b, err := json.Marshal(data)
	return to, kind, b, err
}

// Run deletes, every hour until ctx ends, what has had no event for 90 days (RF-44).
func Run(ctx context.Context, store *db.Store, logger *slog.Logger) {
	prune := func() {
		before := time.Now().Add(-Retention)
		if err := store.PruneNotifications(ctx, before); err != nil {
			logger.Error("limpiar notificaciones", slog.Any("error", err))
		}
		if err := store.PruneNotifiedEvents(ctx, before); err != nil {
			logger.Error("limpiar eventos avisados", slog.Any("error", err))
		}
	}
	prune()
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			prune()
		}
	}
}

// Retention is how long a notification lives after its last event (RF-44).
const Retention = 90 * 24 * time.Hour
