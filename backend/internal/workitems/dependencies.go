package workitems

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/auth"
	"openscum/internal/authz"
	"openscum/internal/db/sqlc"
	"openscum/internal/projects"
)

// dependencies lists the dependencies of a work item from its point of view.
func dependencies(ctx context.Context, q *sqlc.Queries, row sqlc.WorkItem) ([]Dependency, error) {
	deps, err := q.ListDependencies(ctx, row.ID)
	if err != nil {
		return nil, fmt.Errorf("leer dependencias: %w", err)
	}
	if len(deps) == 0 {
		return []Dependency{}, nil
	}
	ids := make([]uuid.UUID, 0, len(deps)+1)
	ids = append(ids, row.ID)
	for _, d := range deps {
		ids = append(ids, d.FromID, d.ToID)
	}
	rows, err := q.WorkItemRefs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("leer referencias: %w", err)
	}
	byID := make(map[uuid.UUID]sqlc.WorkItemRefsRow, len(rows))
	for _, r := range rows {
		byID[r.ID] = r
	}
	out := make([]Dependency, 0, len(deps))
	for _, d := range deps {
		dep := Dependency{ID: d.ID, Type: d.Type, LagDays: d.LagDays, Direction: "outgoing"}
		other := d.ToID
		if d.ToID == row.ID {
			dep.Direction, other = "incoming", d.FromID
		}
		o := byID[other]
		dep.Other = Ref{ID: o.ID, Key: Key(o.Identifier, o.SequenceNumber), Title: o.Title}
		if d.Type == "precedes" {
			// The successor must not start before the predecessor ends plus the lag (RF-96).
			pred, succ := byID[d.FromID], byID[d.ToID]
			if pred.DueDate != nil && succ.StartDate != nil {
				dep.DatesViolated = succ.StartDate.Before(pred.DueDate.AddDate(0, 0, int(d.LagDays)))
			}
		}
		out = append(out, dep)
	}
	return out, nil
}

// AddDependency links two work items (RF-90 to RF-94). The caller must be able
// to write in the source project; the target only has to be visible.
func (s *Service) AddDependency(ctx context.Context, p *authz.Principal, fromID uuid.UUID, req dependencyRequest) ([]Dependency, error) {
	if req.Type != "precedes" && req.LagDays != 0 {
		return nil, apperr.ErrValidation.WithDetail("field", "lag_days")
	}
	var out []Dependency
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		from, project, err := Load(ctx, q, p, fromID, false)
		if err != nil {
			return err
		}
		if !p.CanWriteWorkItems(projects.Ref(project)) {
			return apperr.ErrForbidden
		}
		if err := projects.Writable(project); err != nil {
			return err
		}
		to, _, err := Load(ctx, q, p, req.ToID, false)
		if errors.Is(err, apperr.ErrNotFound) {
			return apperr.ErrValidation.WithDetail("field", "to_id")
		}
		if err != nil {
			return err
		}
		if from.ID == to.ID {
			return errDepHierarchy
		}
		if from.ProjectID == to.ProjectID {
			for _, pair := range [][2]uuid.UUID{{from.ID, to.ID}, {to.ID, from.ID}} {
				inside, err := q.WorkItemIsDescendant(ctx, sqlc.WorkItemIsDescendantParams{WorkItemID: pair[0], Candidate: pair[1]})
				if err != nil {
					return fmt.Errorf("comprobar jerarquía: %w", err)
				}
				if inside {
					return errDepHierarchy
				}
			}
		}
		if req.Type == "blocks" || req.Type == "precedes" {
			// One lock per type: dependencies cross projects (RF-94).
			if err := q.LockDependencies(ctx, req.Type); err != nil {
				return fmt.Errorf("bloquear dependencias: %w", err)
			}
			cycle, err := q.DependencyReaches(ctx, sqlc.DependencyReachesParams{Source: to.ID, Type: req.Type, Target: from.ID})
			if err != nil {
				return fmt.Errorf("comprobar ciclo: %w", err)
			}
			if cycle {
				return errDepCycle
			}
		}
		dep, err := q.CreateDependency(ctx, sqlc.CreateDependencyParams{
			FromID: from.ID, ToID: to.ID, Type: req.Type, LagDays: req.LagDays, CreatedBy: p.UserID,
		})
		if auth.IsUniqueViolation(err) {
			return errDepConflict
		}
		if err != nil {
			return fmt.Errorf("crear dependencia: %w", err)
		}
		if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "work_item.dependency_added", TargetType: "work_item",
			TargetID: from.ID.String(), After: map[string]any{"id": dep.ID, "to_id": to.ID, "type": dep.Type, "lag_days": dep.LagDays}}); err != nil {
			return err
		}
		out, err = dependencies(ctx, q, from)
		return err
	})
	return out, err
}

// RemoveDependency deletes a dependency; the source project must be writable.
func (s *Service) RemoveDependency(ctx context.Context, p *authz.Principal, id uuid.UUID) error {
	return s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		dep, err := q.GetDependency(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("leer dependencia: %w", err)
		}
		from, project, err := Load(ctx, q, p, dep.FromID, false)
		if err != nil {
			return err
		}
		if !p.CanWriteWorkItems(projects.Ref(project)) {
			return apperr.ErrForbidden
		}
		if err := projects.Writable(project); err != nil {
			return err
		}
		if err := q.DeleteDependency(ctx, id); err != nil {
			return fmt.Errorf("borrar dependencia: %w", err)
		}
		return audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "work_item.dependency_removed", TargetType: "work_item",
			TargetID: from.ID.String(), Before: map[string]any{"id": dep.ID, "to_id": dep.ToID, "type": dep.Type}})
	})
}
