package pages

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/authz"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
	"openscum/internal/files"
	"openscum/internal/mentions"
	"openscum/internal/projects"
	"openscum/internal/richtext"
)

type Service struct {
	store *db.Store
	files *files.Service
}

func NewService(store *db.Store, filesSvc *files.Service) *Service {
	return &Service{store: store, files: filesSvc}
}

// pageConflict rejects an autosave over a page someone else changed (RF-34)
// and tells the client the current page version.
func pageConflict(current int32) *apperr.Error {
	return apperr.New(http.StatusConflict, "page_conflict", "La página cambió desde que la abriste").
		WithDetail("page_version", strconv.Itoa(int(current)))
}

type saveRequest struct {
	Page        json.RawMessage `json:"page"`
	PageVersion int32           `json:"page_version" validate:"required,min=1"`
}

// Saved is the page after an autosave.
type Saved struct {
	Page        json.RawMessage `json:"page"`
	PageVersion int32           `json:"page_version"`
	// Version is the owner's general version, which a save also moves, so a
	// later PATCH with the old one still sees the change.
	Version int32                   `json:"version"`
	Refs    map[string]mentions.Ref `json:"refs"`
}

// Save stores the page of kind/id. Only the page version is compared, so a
// change of status or any other field meanwhile is no conflict (RF-34, RF-59).
func (s *Service) Save(ctx context.Context, p *authz.Principal, kind string, id uuid.UUID, req saveRequest) (Saved, error) {
	var out Saved
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		o, err := writable(ctx, q, p, kind, id, true)
		if err != nil {
			return err
		}
		if req.PageVersion != o.pageVersion {
			return pageConflict(o.pageVersion)
		}
		doc, text, _, err := files.Page(ctx, q, o.Owner, req.Page)
		if err != nil {
			return err
		}
		if out, err = update(ctx, q, kind, id, req.PageVersion, doc, text); err != nil {
			return err
		}
		if out.Refs, err = mentions.Refs(ctx, q, p, out.Page); err != nil {
			return err
		}
		if !richtext.Changed(o.description, doc) {
			return nil
		}
		// Only that it changed, never its content (RF-47).
		return audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: kind + ".page_updated", TargetType: kind,
			TargetID: id.String(), After: map[string]any{"description_changed": true}})
	})
	return out, err
}

func update(ctx context.Context, q *sqlc.Queries, kind string, id uuid.UUID, pageVersion int32, doc json.RawMessage, text string) (Saved, error) {
	var (
		out Saved
		err error
	)
	switch kind {
	case KindWorkItem:
		var w sqlc.WorkItem
		w, err = q.UpdateWorkItemPage(ctx, sqlc.UpdateWorkItemPageParams{ID: id, PageVersion: pageVersion, Description: doc, DescriptionText: text})
		out = Saved{Page: w.Description, PageVersion: w.PageVersion, Version: w.Version}
	case KindMilestone:
		var m sqlc.Milestone
		m, err = q.UpdateMilestonePage(ctx, sqlc.UpdateMilestonePageParams{ID: id, PageVersion: pageVersion, Description: doc, DescriptionText: text})
		out = Saved{Page: m.Description, PageVersion: m.PageVersion, Version: m.Version}
	case KindProject:
		var pr sqlc.Project
		pr, err = q.UpdateProjectPage(ctx, sqlc.UpdateProjectPageParams{ID: id, PageVersion: pageVersion, Description: doc, DescriptionText: text})
		out = Saved{Page: pr.Description, PageVersion: pr.PageVersion, Version: pr.Version}
	}
	if err != nil {
		return out, fmt.Errorf("guardar página: %w", err)
	}
	return out, nil
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

// Open returns a file and its content to whoever can read its project, as
// its page (RF-23). A file marked as deleted is not served (RF-49).
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
	r, err := s.files.Read(ctx, f)
	return f, r, err
}
