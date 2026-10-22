// Package db wires the connection pool with the sqlc queries.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"openscum/internal/db/sqlc"
)

// Store gives repositories the pool, the queries and transactions.
type Store struct {
	Pool *pgxpool.Pool
	*sqlc.Queries
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{Pool: pool, Queries: sqlc.New(pool)}
}

// InTx runs fn inside a transaction; it commits only if fn returns nil.
func (s *Store) InTx(ctx context.Context, fn func(tx pgx.Tx, q *sqlc.Queries) error) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(tx, s.Queries.WithTx(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
