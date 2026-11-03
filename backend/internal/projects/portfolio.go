package projects

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/authz"
	"openscum/internal/db/sqlc"
)

var errTemplateProgram = apperr.New(400, "template_program", "Una plantilla no puede ser un programa")

type portfolioRequest struct {
	Version   int32  `json:"version" validate:"required"`
	IsProgram bool   `json:"is_program"`
	Weight    *int32 `json:"weight" validate:"omitempty,min=0,max=100000"` // null: its effort (RF-6)
}

// SetPortfolio marks or unmarks a program and sets or clears the weight of a
// project, for the system administrator (spec 010, RF-1 to RF-10).
func (s *Service) SetPortfolio(ctx context.Context, p *authz.Principal, id uuid.UUID, req portfolioRequest) (Project, error) {
	if !p.Has(authz.PortfolioManage) {
		return Project{}, apperr.ErrForbidden
	}
	var out Project
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		row, err := Get(ctx, q, p, id, true)
		if err != nil {
			return err
		}
		if err := Writable(row); err != nil {
			return err
		}
		if req.Version != row.Version {
			return apperr.VersionConflict(row.Version)
		}
		if req.IsProgram && row.IsTemplate { // RF-4
			return errTemplateProgram
		}
		next, err := q.SetProjectPortfolio(ctx, sqlc.SetProjectPortfolioParams{ID: id, IsProgram: req.IsProgram, PortfolioWeight: req.Weight})
		if err != nil {
			return fmt.Errorf("guardar programa: %w", err)
		}
		if row.IsProgram != next.IsProgram { // RF-9
			if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "project.program_set", TargetType: "project",
				TargetID: id.String(), Before: map[string]any{"is_program": row.IsProgram}, After: map[string]any{"is_program": next.IsProgram}}); err != nil {
				return err
			}
		}
		if !sameWeight(row.PortfolioWeight, next.PortfolioWeight) { // RF-10
			if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "project.portfolio_weight_changed", TargetType: "project",
				TargetID: id.String(), Before: map[string]any{"weight": row.PortfolioWeight}, After: map[string]any{"weight": next.PortfolioWeight}}); err != nil {
				return err
			}
		}
		out, err = view(ctx, q, p, next)
		return err
	})
	return out, err
}

func sameWeight(a, b *int32) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
