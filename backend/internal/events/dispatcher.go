package events

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"openscum/internal/db"
	"openscum/internal/db/sqlc"
)

// Consumer handles one event. Returning an error stops delivery to that
// consumer until the next poll, which retries the same event.
type Consumer func(ctx context.Context, e Event) error

const batchSize = 100

// Dispatcher polls the outbox and feeds every registered consumer from its
// own cursor, advancing it only after the consumer succeeds.
type Dispatcher struct {
	store    *db.Store
	logger   *slog.Logger
	interval time.Duration

	mu        sync.Mutex
	consumers map[string]Consumer
}

func NewDispatcher(store *db.Store, logger *slog.Logger, interval time.Duration) *Dispatcher {
	return &Dispatcher{store: store, logger: logger, interval: interval, consumers: map[string]Consumer{}}
}

// Register adds a consumer under a stable name; its cursor survives restarts.
func (d *Dispatcher) Register(name string, c Consumer) {
	d.mu.Lock()
	d.consumers[name] = c
	d.mu.Unlock()
}

// Run polls until ctx is done.
func (d *Dispatcher) Run(ctx context.Context) {
	t := time.NewTicker(d.interval)
	defer t.Stop()
	for {
		d.Poll(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Poll delivers every pending event once; tests call it directly.
func (d *Dispatcher) Poll(ctx context.Context) {
	d.mu.Lock()
	consumers := make(map[string]Consumer, len(d.consumers))
	for k, v := range d.consumers {
		consumers[k] = v
	}
	d.mu.Unlock()
	for name, c := range consumers {
		if err := d.deliver(ctx, name, c); err != nil && ctx.Err() == nil {
			d.logger.Warn("entrega de eventos detenida", slog.String("consumer", name), slog.String("error", err.Error()))
		}
	}
}

func (d *Dispatcher) deliver(ctx context.Context, name string, c Consumer) error {
	cursor, err := d.store.EnsureEventConsumer(ctx, name)
	if err != nil {
		return fmt.Errorf("leer cursor: %w", err)
	}
	for {
		rows, err := d.store.ListDomainEventsAfter(ctx, sqlc.ListDomainEventsAfterParams{ID: cursor, Limit: batchSize})
		if err != nil {
			return fmt.Errorf("leer eventos: %w", err)
		}
		for _, r := range rows {
			if err := c(ctx, fromRow(r)); err != nil {
				return fmt.Errorf("evento %d: %w", r.ID, err)
			}
			if err := d.store.AdvanceEventConsumer(ctx, sqlc.AdvanceEventConsumerParams{Name: name, LastEventID: r.ID}); err != nil {
				return fmt.Errorf("avanzar cursor: %w", err)
			}
			cursor = r.ID
		}
		if len(rows) < batchSize {
			return nil
		}
	}
}
