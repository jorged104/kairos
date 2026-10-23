package events

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/db"
	"openscum/internal/db/dbtest"
	"openscum/internal/db/sqlc"
)

func newEvent() New {
	return New{Type: WorkItemClosed, ProjectID: uuid.New(), SubjectType: "work_item", SubjectID: uuid.New(), Payload: map[string]any{"k": 1}}
}

func emitTx(t *testing.T, store *db.Store, fail bool) {
	t.Helper()
	errRollback := errors.New("rollback")
	err := store.InTx(context.Background(), func(_ pgx.Tx, q *sqlc.Queries) error {
		if err := Emit(context.Background(), q, newEvent()); err != nil {
			return err
		}
		if fail {
			return errRollback
		}
		return nil
	})
	if err != nil && !errors.Is(err, errRollback) {
		t.Fatal(err)
	}
}

type recorder struct {
	seen   []Event
	failOn int // fail the first delivery of this many-th event (1-based), once
}

func (r *recorder) consume(_ context.Context, e Event) error {
	if r.failOn == len(r.seen)+1 {
		r.failOn = 0
		return errors.New("consumer down")
	}
	r.seen = append(r.seen, e)
	return nil
}

func TestEmitIsPartOfTheTransaction(t *testing.T) {
	store := db.NewStore(dbtest.New(t))
	emitTx(t, store, true)
	emitTx(t, store, false)
	n, err := store.CountDomainEvents(context.Background(), sqlc.CountDomainEventsParams{})
	if err != nil || n != 1 {
		t.Fatalf("events = %d, %v; want only the committed one (RF-120)", n, err)
	}
}

func TestDispatcherDeliversInOrderAndRetries(t *testing.T) {
	ctx := context.Background()
	store := db.NewStore(dbtest.New(t))
	d := NewDispatcher(store, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Hour)
	rec := &recorder{failOn: 2}
	d.Register("test", rec.consume)

	for range 3 {
		emitTx(t, store, false)
	}
	d.Poll(ctx)
	if len(rec.seen) != 1 {
		t.Fatalf("delivered %d events before the failure, want 1", len(rec.seen))
	}
	d.Poll(ctx) // retries the failed event, then the rest
	if len(rec.seen) != 3 {
		t.Fatalf("delivered %d events, want 3 (RF-121)", len(rec.seen))
	}
	for i := 1; i < len(rec.seen); i++ {
		if rec.seen[i].ID <= rec.seen[i-1].ID {
			t.Fatalf("out of order: %d after %d", rec.seen[i].ID, rec.seen[i-1].ID)
		}
	}
	d.Poll(ctx)
	if len(rec.seen) != 3 {
		t.Fatal("events delivered twice after a successful delivery")
	}

	// A second consumer starts from the beginning with its own cursor.
	other := &recorder{}
	d.Register("other", other.consume)
	d.Poll(ctx)
	if len(other.seen) != 3 || len(rec.seen) != 3 {
		t.Fatalf("other = %d, test = %d; want 3 and 3", len(other.seen), len(rec.seen))
	}
}

func TestSlowTransactionIsNotSkipped(t *testing.T) {
	ctx := context.Background()
	store := db.NewStore(dbtest.New(t))
	d := NewDispatcher(store, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Hour)
	rec := &recorder{}
	d.Register("test", rec.consume)

	slow, err := store.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = slow.Rollback(ctx) }()
	if err := Emit(ctx, store.WithTx(slow), newEvent()); err != nil {
		t.Fatal(err)
	}

	fast := make(chan error, 1)
	go func() {
		fast <- store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error { return Emit(ctx, q, newEvent()) })
	}()
	select {
	case err := <-fast:
		t.Fatalf("a later event committed while an earlier one was open: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	d.Poll(ctx)
	if len(rec.seen) != 0 {
		t.Fatalf("delivered %d events with the first transaction still open", len(rec.seen))
	}
	if err := slow.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-fast; err != nil {
		t.Fatal(err)
	}
	d.Poll(ctx)
	if len(rec.seen) != 2 || rec.seen[0].ID >= rec.seen[1].ID {
		t.Fatalf("delivered %+v, want both events in order", rec.seen)
	}
}
