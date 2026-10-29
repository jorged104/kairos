package pages

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/authz"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
	"openscum/internal/files"
)

type Service struct {
	store *db.Store
	files *files.Service
}

func NewService(store *db.Store, filesSvc *files.Service) *Service {
	return &Service{store: store, files: filesSvc}
}

// Upload stores a file for the page of kind/id (RF-14, RF-15). The owner is
// not locked: the upload may take a while and only adds a row.
func (s *Service) Upload(ctx context.Context, p *authz.Principal, kind string, id uuid.UUID, name string, data []byte) (files.File, error) {
	var out files.File
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		o, err := writable(ctx, q, p, kind, id, false)
		if err != nil {
			return err
		}
		out, err = s.files.Upload(ctx, q, o.Owner, p.UserID, name, data)
		return err
	})
	return out, err
}
