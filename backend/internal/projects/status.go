package projects

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/authz"
	"openscum/internal/db/sqlc"
	"openscum/internal/mentions"
)

type statusRequest struct {
	Version  int32     `json:"version" validate:"required,min=1"`
	StatusID uuid.UUID `json:"status_id" validate:"required"`
}

// statusOf is the project's status; a null one, left by an older version of
// the API, reads as the initial status (spec 012, RF-13).
func statusOf(ctx context.Context, q *sqlc.Queries, row sqlc.Project) (sqlc.ProjectStatus, error) {
	var (
		st  sqlc.ProjectStatus
		err error
	)
	if row.StatusID != nil {
		st, err = q.GetProjectStatus(ctx, *row.StatusID)
	} else {
		st, err = q.GetInitialProjectStatus(ctx)
	}
	if err != nil {
		return st, fmt.Errorf("leer estado del proyecto: %w", err)
	}
	return st, nil
}

// SetStatus moves a project to another status of the catalog, with no flow
// rules; when the category crosses, it closes or reopens it as well (spec
// 012, RF-17 to RF-25).
func (s *Service) SetStatus(ctx context.Context, p *authz.Principal, id uuid.UUID, req statusRequest) (Project, error) {
	var out Project
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		row, err := Get(ctx, q, p, id, true)
		if err != nil {
			return err
		}
		if row.IsTemplate {
			return apperr.ErrNotFound
		}
		if !p.CanManageProject(Ref(row)) {
			return apperr.ErrForbidden
		}
		if row.Lifecycle == LifecycleArchived { // RF-23
			return ErrArchived
		}
		if req.Version != row.Version {
			return apperr.VersionConflict(row.Version)
		}
		next, err := q.GetProjectStatus(ctx, req.StatusID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && next.RetiredAt != nil) { // RF-10
			return apperr.ErrValidation.WithDetail("field", "status_id")
		}
		if err != nil {
			return fmt.Errorf("leer estado: %w", err)
		}
		cur, err := statusOf(ctx, q, row)
		if err != nil {
			return err
		}
		lifecycle := LifecycleActive
		if next.IsClosed {
			lifecycle = LifecycleClosed
		}
		updated, err := q.SetProjectStatus(ctx, sqlc.SetProjectStatusParams{ID: id, Version: req.Version, StatusID: &next.ID, Lifecycle: lifecycle})
		if err != nil {
			return fmt.Errorf("cambiar estado: %w", err)
		}
		if lifecycle != row.Lifecycle { // RF-19, RF-20
			action := "project.close"
			if lifecycle == LifecycleActive {
				action = "project.reopen"
			}
			if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: action, TargetType: "project",
				TargetID: id.String(), Before: row.Lifecycle, After: lifecycle}); err != nil {
				return err
			}
		}
		if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "project.status_changed", TargetType: "project",
			TargetID: id.String(), Before: map[string]any{"status_id": cur.ID, "name": cur.Name},
			After: map[string]any{"status_id": next.ID, "name": next.Name}}); err != nil { // RF-25
			return err
		}
		out, err = view(ctx, q, p, updated)
		if err != nil {
			return err
		}
		out.Refs, err = mentions.Refs(ctx, q, p, out.Description)
		return err
	})
	return out, err
}

// followLifecycle keeps the status in the category of a new lifecycle: closing
// moves an open status to the closing one, and reopening or unarchiving moves
// a closed one to the initial status (RF-21, RF-22).
func followLifecycle(ctx context.Context, q *sqlc.Queries, row sqlc.Project) (sqlc.Project, error) {
	if row.Lifecycle == LifecycleArchived {
		return row, nil
	}
	cur, err := statusOf(ctx, q, row)
	if err != nil {
		return row, err
	}
	closed := row.Lifecycle == LifecycleClosed
	if cur.IsClosed == closed && row.StatusID != nil {
		return row, nil
	}
	var next sqlc.ProjectStatus
	if closed {
		next, err = q.GetClosingProjectStatus(ctx)
	} else if cur.IsClosed {
		next, err = q.GetInitialProjectStatus(ctx)
	} else {
		next = cur
	}
	if err != nil {
		return row, fmt.Errorf("leer estado: %w", err)
	}
	if err := q.SetProjectStatusID(ctx, sqlc.SetProjectStatusIDParams{ID: row.ID, StatusID: &next.ID}); err != nil {
		return row, fmt.Errorf("cambiar estado: %w", err)
	}
	row.StatusID = &next.ID
	return row, nil
}
