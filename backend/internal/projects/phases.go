package projects

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/authz"
	"openscum/internal/db/sqlc"
	"openscum/internal/workcore"
)

// Phases lists the phases of a project in order (RF-40).
func (s *Service) Phases(ctx context.Context, p *authz.Principal, projectID uuid.UUID) ([]Phase, error) {
	if _, err := Get(ctx, s.store.Queries, p, projectID, false); err != nil {
		return nil, err
	}
	rows, err := s.store.ListProjectPhases(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("listar fases: %w", err)
	}
	out := make([]Phase, 0, len(rows))
	for _, r := range rows {
		out = append(out, toPhase(sqlc.GetProjectPhaseRow(r)))
	}
	return out, nil
}

// phaseChange loads a phase and its writable project for a manager.
func phaseChange(ctx context.Context, q *sqlc.Queries, p *authz.Principal, id uuid.UUID) (sqlc.GetProjectPhaseRow, sqlc.Project, error) {
	ph, err := q.GetProjectPhase(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ph, sqlc.Project{}, apperr.ErrNotFound
	}
	if err != nil {
		return ph, sqlc.Project{}, fmt.Errorf("leer fase: %w", err)
	}
	project, err := Get(ctx, q, p, ph.ProjectID, true)
	if err != nil {
		return ph, project, err
	}
	if !p.CanManageProject(Ref(project)) {
		return ph, project, apperr.ErrForbidden
	}
	return ph, project, Writable(project)
}

// UpdatePhase sets the dates of a phase inside those of its project (RF-41).
func (s *Service) UpdatePhase(ctx context.Context, p *authz.Principal, id uuid.UUID, req phaseRequest) (Phase, error) {
	var out Phase
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		ph, project, err := phaseChange(ctx, q, p, id)
		if err != nil {
			return err
		}
		if err := workcore.CheckRange(req.StartDate, req.FinishDate, errDateRange); err != nil {
			return err
		}
		for _, d := range []*workcore.Date{req.StartDate, req.FinishDate} {
			if d == nil {
				continue
			}
			if (project.StartDate != nil && d.Before(*project.StartDate)) || (project.EndDate != nil && d.After(*project.EndDate)) {
				return errDateRange.WithDetail("field", "phase")
			}
		}
		if err := q.UpdateProjectPhaseDates(ctx, sqlc.UpdateProjectPhaseDatesParams{
			ID: id, StartDate: workcore.ToTime(req.StartDate), FinishDate: workcore.ToTime(req.FinishDate),
		}); err != nil {
			return fmt.Errorf("actualizar fase: %w", err)
		}
		ph.StartDate, ph.FinishDate = workcore.ToTime(req.StartDate), workcore.ToTime(req.FinishDate)
		out = toPhase(ph)
		return audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "phase.updated", TargetType: "project_phase", TargetID: id.String(), After: out})
	})
	return out, err
}

// SetGate marks a gate passed with date and user, or clears it (RF-42). Gates
// never block anything (RF-43).
func (s *Service) SetGate(ctx context.Context, p *authz.Principal, id uuid.UUID, gate string, passed bool) (Phase, error) {
	var out Phase
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		ph, _, err := phaseChange(ctx, q, p, id)
		if err != nil {
			return err
		}
		var at *time.Time
		var by *uuid.UUID
		if passed {
			now, user := time.Now().UTC(), p.UserID
			at, by = &now, &user
		}
		switch {
		case gate == "start" && ph.HasStartGate:
			err = q.SetStartGate(ctx, sqlc.SetStartGateParams{ID: id, PassedAt: at, PassedBy: by})
			ph.StartGatePassedAt, ph.StartGatePassedBy = at, by
		case gate == "finish" && ph.HasFinishGate:
			err = q.SetFinishGate(ctx, sqlc.SetFinishGateParams{ID: id, PassedAt: at, PassedBy: by})
			ph.FinishGatePassedAt, ph.FinishGatePassedBy = at, by
		default:
			return apperr.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("marcar gate: %w", err)
		}
		out = toPhase(ph)
		return audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "phase.gate_" + gate, TargetType: "project_phase",
			TargetID: id.String(), After: map[string]bool{"passed": passed}})
	})
	return out, err
}

func toPhase(r sqlc.GetProjectPhaseRow) Phase {
	return Phase{
		ID: r.ID, DefinitionID: r.DefinitionID, Name: r.Name, Position: r.Position,
		StartDate: workcore.FromTime(r.StartDate), FinishDate: workcore.FromTime(r.FinishDate),
		HasStartGate: r.HasStartGate, HasFinishGate: r.HasFinishGate,
		StartGatePassedAt: r.StartGatePassedAt, StartGatePassedBy: r.StartGatePassedBy,
		FinishGatePassedAt: r.FinishGatePassedAt, FinishGatePassedBy: r.FinishGatePassedBy,
	}
}
