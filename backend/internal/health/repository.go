package health

import (
	"context"

	"openscum/internal/db/sqlc"
)

type Repository struct {
	q *sqlc.Queries
}

func NewRepository(q *sqlc.Queries) *Repository {
	return &Repository{q: q}
}

func (r *Repository) Ping(ctx context.Context) error {
	_, err := r.q.Ping(ctx)
	return err
}
