package projects

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/authz"
	"openscum/internal/db/sqlc"
	"openscum/internal/schedule"
	"openscum/internal/workcore"
)

// Baseline is one version of the baseline of a project (RF-3, RF-5, RF-6).
type Baseline struct {
	Version   int32        `json:"version"`
	Reason    *string      `json:"reason"`
	CreatedAt time.Time    `json:"created_at"`
	Author    workcore.Ref `json:"author"`
}

// Schedule is the plan of a project and each of its work items (spec 009).
type Schedule struct {
	Today    string         `json:"today"`
	Baseline *Baseline      `json:"baseline"` // the current one, or null
	Versions []Baseline     `json:"versions"` // the earlier ones, newest first
	Items    []ItemSchedule `json:"items"`
}

// ItemSchedule is the plan of a work item and how it stands against the
// baseline (RF-11, RF-12).
type ItemSchedule struct {
	ID         uuid.UUID        `json:"id"`
	Figures    schedule.Figures `json:"figures"`
	InBaseline bool             `json:"in_baseline"`
	BaseStart  *workcore.Date   `json:"base_start"`
	BaseDue    *workcore.Date   `json:"base_due"`
	// Changed when the current dates differ from the baseline ones; the shift
	// is in days, of the end if both have one and else of the start.
	Changed   bool `json:"changed"`
	ShiftDays *int `json:"shift_days"`
}

type baselineRequest struct {
	Reason *string `json:"reason"`
}

func (r *baselineRequest) Normalize() {
	if r.Reason != nil {
		v := strings.TrimSpace(*r.Reason)
		r.Reason = &v
		if v == "" {
			r.Reason = nil
		}
	}
}

// SetBaseline freezes the dates of every work item of the project as a new
// version (RF-1 to RF-3, RF-7, RF-8, RF-49).
func (s *Service) SetBaseline(ctx context.Context, p *authz.Principal, id uuid.UUID, req baselineRequest) (Baseline, error) {
	if req.Reason != nil && len([]rune(*req.Reason)) > 500 {
		return Baseline{}, apperr.ErrValidation.WithDetail("field", "reason")
	}
	var out Baseline
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		if _, err := s.teamChange(ctx, q, p, id); err != nil {
			return err
		}
		last, err := q.LatestBaselineVersion(ctx, id)
		if err != nil {
			return fmt.Errorf("versión de la línea base: %w", err)
		}
		if last > 0 && req.Reason == nil { // RF-2
			return apperr.ErrValidation.WithDetail("field", "reason")
		}
		b, err := q.CreateBaseline(ctx, sqlc.CreateBaselineParams{ProjectID: id, Version: last + 1, Reason: req.Reason, CreatedBy: p.UserID})
		if err != nil {
			return fmt.Errorf("crear línea base: %w", err)
		}
		n, err := q.CopyBaselineItems(ctx, sqlc.CopyBaselineItemsParams{BaselineID: b.ID, ProjectID: id})
		if err != nil {
			return fmt.Errorf("copiar fechas: %w", err)
		}
		u, err := q.GetUser(ctx, p.UserID)
		if err != nil {
			return fmt.Errorf("autor: %w", err)
		}
		out = Baseline{Version: b.Version, Reason: b.Reason, CreatedAt: b.CreatedAt, Author: workcore.Ref{ID: u.ID, Name: u.Name}}
		return audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "project.baseline_set", TargetType: "project",
			TargetID: id.String(), After: map[string]any{"version": b.Version, "reason": b.Reason, "work_items": n}})
	})
	return out, err
}

// Schedule is the baseline of a project and the plan of its work items at
// today's date, for whoever sees the project.
func (s *Service) Schedule(ctx context.Context, p *authz.Principal, id uuid.UUID, today time.Time) (Schedule, error) {
	q := s.store.Queries
	if _, err := Get(ctx, q, p, id, false); err != nil {
		return Schedule{}, err
	}
	out := Schedule{Today: today.Format(time.DateOnly), Versions: []Baseline{}, Items: []ItemSchedule{}}
	versions, err := q.ListBaselines(ctx, id)
	if err != nil {
		return out, fmt.Errorf("líneas base: %w", err)
	}
	for i, v := range versions {
		b := Baseline{Version: v.Version, Reason: v.Reason, CreatedAt: v.CreatedAt, Author: workcore.Ref{ID: v.AuthorID, Name: v.AuthorName}}
		if i == 0 {
			out.Baseline = &b
		} else {
			out.Versions = append(out.Versions, b)
		}
	}
	plan, err := schedule.LoadProject(ctx, q, id, today)
	if err != nil {
		return out, err
	}
	for _, r := range plan.Rows {
		out.Items = append(out.Items, ItemScheduleOf(plan, r))
	}
	return out, nil
}

// ItemScheduleOf is the plan of one work item of a loaded project; a closed
// one shows no plan (RF-34).
func ItemScheduleOf(plan schedule.ProjectPlan, r sqlc.ProjectPlanItemsRow) ItemSchedule {
	it := ItemSchedule{ID: r.ID, Figures: plan.ByItem[r.ID], InBaseline: r.InBaseline}
	if r.IsClosed {
		it.Figures = schedule.Figures{Real: 100}
	}
	if !r.InBaseline {
		return it
	}
	it.BaseStart, it.BaseDue = workcore.FromTime(r.BaseStart), workcore.FromTime(r.BaseDue)
	it.Changed = !sameDay(r.StartDate, r.BaseStart) || !sameDay(r.DueDate, r.BaseDue)
	if it.Changed {
		switch {
		case r.DueDate != nil && r.BaseDue != nil:
			d := schedule.Days(*r.BaseDue, *r.DueDate)
			it.ShiftDays = &d
		case r.StartDate != nil && r.BaseStart != nil:
			d := schedule.Days(*r.BaseStart, *r.StartDate)
			it.ShiftDays = &d
		}
	}
	return it
}

func sameDay(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return schedule.Days(*a, *b) == 0
}
