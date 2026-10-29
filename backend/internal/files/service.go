// Package files keeps the images and attachments of the pages of spec 005
// (RF-14 to RF-24, RF-49 to RF-55). Who may upload to an owner is checked by
// the pages package; this one stores, serves and tracks the files.
package files

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/authz"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
	"openscum/internal/projects"
	"openscum/internal/richtext"
)

// Owner is the project, milestone or work item whose page holds the files.
type Owner struct {
	Kind      string // project, milestone or work_item
	ID        uuid.UUID
	ProjectID uuid.UUID
}

// File is a file as the API returns it.
type File struct {
	ID          uuid.UUID `json:"id"`
	Kind        string    `json:"kind"`
	Name        string    `json:"name"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type"`
}

func view(f sqlc.PageFile) File {
	return File{ID: f.ID, Kind: f.Kind, Name: f.Name, Size: f.SizeBytes, ContentType: f.ContentType}
}

type Service struct {
	store   *db.Store
	storage *Storage
}

func NewService(store *db.Store, storage *Storage) *Service {
	return &Service{store: store, storage: storage}
}

// Upload stores one file of o, uploaded by by, inside the caller's transaction.
func (s *Service) Upload(ctx context.Context, q *sqlc.Queries, o Owner, by uuid.UUID, name string, data []byte) (File, error) {
	if len(data) > MaxBytes {
		return File{}, ErrFileTooLarge
	}
	name = cleanName(name)
	kind, contentType, err := detect(name, data)
	if err != nil {
		return File{}, err
	}
	f, err := s.storage.save(ctx, q, sqlc.CreatePageFileParams{
		ProjectID: o.ProjectID, OwnerKind: o.Kind, OwnerID: o.ID, Kind: kind, Name: name,
		ContentType: contentType, SizeBytes: int64(len(data)), CreatedBy: by,
	}, data)
	if err != nil {
		return File{}, err
	}
	return view(f), nil
}

// Owned returns every file of o, marked ones too, as richtext.CleanPage
// expects them: a saved page may bring a marked file back (RF-50).
func Owned(ctx context.Context, q *sqlc.Queries, o Owner) (map[uuid.UUID]richtext.File, error) {
	rows, err := q.ListOwnerPageFiles(ctx, sqlc.ListOwnerPageFilesParams{OwnerKind: o.Kind, OwnerID: o.ID})
	if err != nil {
		return nil, fmt.Errorf("listar archivos: %w", err)
	}
	out := make(map[uuid.UUID]richtext.File, len(rows))
	for _, f := range rows {
		out[f.ID] = richtext.File{Name: f.Name, Size: f.SizeBytes, Kind: f.Kind}
	}
	return out, nil
}

// Sync leaves live the files of the saved page and marks the rest of o's as
// deleted (RF-49, RF-50). Nothing is removed from the storage (RF-52).
func Sync(ctx context.Context, q *sqlc.Queries, o Owner, live []uuid.UUID) error {
	if live == nil {
		live = []uuid.UUID{}
	}
	if err := q.SyncPageFiles(ctx, sqlc.SyncPageFilesParams{Live: live, OwnerKind: o.Kind, OwnerID: o.ID}); err != nil {
		return fmt.Errorf("marcar archivos: %w", err)
	}
	return nil
}

// Copy duplicates the live files of from for to (RF-53) and returns the new id
// of each copied file, to rewrite the copied page.
func (s *Service) Copy(ctx context.Context, q *sqlc.Queries, from, to Owner, by uuid.UUID) (map[uuid.UUID]uuid.UUID, error) {
	rows, err := q.ListOwnerPageFiles(ctx, sqlc.ListOwnerPageFilesParams{OwnerKind: from.Kind, OwnerID: from.ID})
	if err != nil {
		return nil, fmt.Errorf("listar archivos: %w", err)
	}
	ids := map[uuid.UUID]uuid.UUID{}
	for _, f := range rows {
		if f.DeletedAt != nil {
			continue
		}
		c, err := s.storage.copy(ctx, q, f, sqlc.CopyPageFileParams{ProjectID: to.ProjectID, OwnerID: to.ID, CreatedBy: by})
		if err != nil {
			return nil, err
		}
		ids[f.ID] = c.ID
	}
	return ids, nil
}

// Open returns a file and its content to whoever can read its project
// (RF-23). A marked file is not served (RF-49).
func (s *Service) Open(ctx context.Context, p *authz.Principal, id uuid.UUID) (sqlc.PageFile, io.ReadCloser, error) {
	f, err := s.store.GetPageFile(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && f.DeletedAt != nil) {
		return f, nil, apperr.ErrNotFound
	}
	if err != nil {
		return f, nil, fmt.Errorf("leer archivo: %w", err)
	}
	if _, err := projects.Get(ctx, s.store.Queries, p, f.ProjectID, false); err != nil {
		return f, nil, err
	}
	r, err := s.storage.open(ctx, s.store.Queries, f)
	return f, r, err
}
