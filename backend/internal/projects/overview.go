package projects

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/authz"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
	"openscum/internal/portfolio"
	"openscum/internal/schedule"
	"openscum/internal/workcore"
)

// Overview is the project at a glance (spec 008, RF-6 to RF-17).
type Overview struct {
	Today string `json:"today"`
	// Weighted by weight, closed ones at 100 % (spec 008, Conceptos).
	Progress int `json:"progress"`
	// Plan, real, gap and level (spec 009, RF-44); has_plan, expected and
	// deviation are the same rounded, for the earlier API.
	Figures    schedule.Figures `json:"figures"`
	PlanSeries []PlanPoint      `json:"plan_series"` // RF-46
	HasPlan    bool             `json:"has_plan"`    // false: «Sin plazo» (spec 009, RF-24)
	Expected   int              `json:"expected"`
	Deviation  int              `json:"deviation"`

	Statuses []StatusCount `json:"statuses"`
	Total    int           `json:"total"`
	Closed   int           `json:"closed"`
	Overdue  int           `json:"overdue"`
	Blocked  int           `json:"blocked"`

	NextMilestone *MilestoneLine  `json:"next_milestone"`
	Milestones    []MilestoneLine `json:"milestones"`
	Risks         []Risk          `json:"risks"`
	Load          []Member        `json:"load"`
	History       []DayProgress   `json:"history"`
	// The aggregate and the direct children of a program (spec 010, RF-39, RF-40).
	Program *portfolio.Card `json:"program,omitempty"`
}

type StatusCount struct {
	StatusID uuid.UUID `json:"status_id"`
	Count    int       `json:"count"`
}

type MilestoneLine struct {
	ID       uuid.UUID      `json:"id"`
	Name     string         `json:"name"`
	DueDate  *workcore.Date `json:"due_date"`
	Status   string         `json:"status"`
	Percent  int            `json:"percent"`
	Closed   int32          `json:"closed"`
	Total    int32          `json:"total"`
	DaysLeft *int           `json:"days_left,omitempty"`
	// Spec 009, RF-45; no plan once met or cancelled (RF-34).
	Figures schedule.Figures `json:"figures"`
}

// Risk is an open work item that is overdue, blocked or behind (RF-15).
type Risk struct {
	ID        uuid.UUID      `json:"id"`
	Key       string         `json:"key"`
	Title     string         `json:"title"`
	Reason    string         `json:"reason"`    // overdue, blocked or behind
	Days      int            `json:"days"`      // overdue
	Deviation int            `json:"deviation"` // the gap, rounded
	Gap       float64        `json:"gap"`
	Level     schedule.Level `json:"level,omitempty"` // behind: at_risk or late (spec 009, RF-47)
	DueDate   *workcore.Date `json:"due_date"`
	Assignees []workcore.Ref `json:"assignees"`
}

// PlanPoint is the plan of one day, for the chart (spec 009, RF-46).
type PlanPoint struct {
	Day  workcore.Date `json:"day"`
	Plan float64       `json:"plan"`
}

type DayProgress struct {
	Day      workcore.Date `json:"day"`
	Progress int16         `json:"progress"`
}

const (
	maxRisks      = 10
	maxLoad       = 5
	maxPlanPoints = 400
)

func (s *Service) Overview(ctx context.Context, p *authz.Principal, id uuid.UUID, today time.Time) (Overview, error) {
	q := s.store.Queries
	row, err := Get(ctx, q, p, id, false)
	if err != nil {
		return Overview{}, err
	}
	out := Overview{Today: today.Format(time.DateOnly), Statuses: []StatusCount{}, Milestones: []MilestoneLine{},
		Risks: []Risk{}, Load: []Member{}, History: []DayProgress{}, PlanSeries: []PlanPoint{}}
	progress, err := q.ProjectProgress(ctx, id)
	if err != nil {
		return out, fmt.Errorf("avance: %w", err)
	}
	out.Progress = int(progress)
	plan, err := schedule.LoadProject(ctx, q, id, today)
	if err != nil {
		return out, err
	}
	if row.Lifecycle == LifecycleActive { // RF-34
		value, ok := projectPlan(row, plan.Items, today)
		out.Figures = schedule.Make(value, ok, plan.Real, plan.Settings)
	} else {
		out.Figures = schedule.Figures{Real: plan.Real}
	}
	if out.Figures.HasPlan {
		out.HasPlan = true
		out.Expected = int(math.Round(out.Figures.Plan))
		out.Deviation = out.Progress - out.Expected
	}

	counts, err := q.ProjectStatusCounts(ctx, id)
	if err != nil {
		return out, fmt.Errorf("conteo por estado: %w", err)
	}
	closedIDs, err := q.ClosedStatusIDs(ctx)
	if err != nil {
		return out, fmt.Errorf("estados cerrados: %w", err)
	}
	for _, c := range counts {
		out.Statuses = append(out.Statuses, StatusCount{StatusID: c.StatusID, Count: int(c.Items)})
		out.Total += int(c.Items)
		for _, cl := range closedIDs {
			if cl == c.StatusID {
				out.Closed += int(c.Items)
			}
		}
	}

	if err := s.overviewMilestones(ctx, q, id, today, plan, &out); err != nil {
		return out, err
	}
	if err := s.overviewRisks(ctx, q, row, today, plan, &out); err != nil {
		return out, err
	}
	team, err := members(ctx, q, row)
	if err != nil {
		return out, err
	}
	for _, m := range team {
		if len(out.Load) < maxLoad && (m.Load.Total > 0 || m.Capacity != nil) {
			out.Load = append(out.Load, m)
		}
	}
	history, err := q.ListDailyProgress(ctx, sqlc.ListDailyProgressParams{ProjectID: id, Since: today.AddDate(-1, 0, 0)})
	if err != nil {
		return out, fmt.Errorf("avance diario: %w", err)
	}
	for _, h := range history {
		out.History = append(out.History, DayProgress{Day: workcore.Date{Time: h.Day}, Progress: h.Progress})
	}
	if out.HasPlan {
		out.PlanSeries = planSeries(row, plan.Items, history, today)
	}
	if row.IsProgram {
		tree, set, err := portfolio.Load(ctx, q, today)
		if err != nil {
			return out, err
		}
		card := tree.AsSeen(tree.ProgramCard(row.ID, set), p)
		out.Program = &card
	}
	return out, nil
}

func projectPlan(row sqlc.Project, items []schedule.Item, day time.Time) (float64, bool) {
	return schedule.ProjectValue(row.StartDate, row.EndDate, items, day)
}

// planSeries is the plan of each day of the chart: from the start of the
// project (or the first day recorded) to its end (or the last planned end),
// at most maxPlanPoints days apart evenly (spec 009, RF-46).
func planSeries(row sqlc.Project, items []schedule.Item, history []sqlc.ListDailyProgressRow, today time.Time) []PlanPoint {
	from, to := today, today
	switch {
	case row.StartDate != nil:
		from = *row.StartDate
	case len(history) > 0:
		from = history[0].Day
	}
	if row.EndDate != nil {
		to = *row.EndDate
	} else {
		for _, it := range items {
			if it.Due != nil && it.Due.After(to) {
				to = *it.Due
			}
		}
	}
	if to.Before(from) {
		from, to = to, from
	}
	span := schedule.Days(from, to)
	step := max(1, (span+maxPlanPoints-1)/maxPlanPoints)
	out := make([]PlanPoint, 0, span/step+2)
	for d := 0; ; d += step {
		if d > span {
			d = span
		}
		day := from.AddDate(0, 0, d)
		v, _ := projectPlan(row, items, day)
		out = append(out, PlanPoint{Day: workcore.Date{Time: day}, Plan: math.Round(v*10) / 10})
		if d == span {
			return out
		}
	}
}

func (s *Service) overviewMilestones(ctx context.Context, q *sqlc.Queries, id uuid.UUID, today time.Time, plan schedule.ProjectPlan, out *Overview) error {
	ms, err := q.ListProjectMilestones(ctx, id)
	if err != nil {
		return fmt.Errorf("hitos: %w", err)
	}
	progress, err := q.MilestoneProgress(ctx, sqlc.MilestoneProgressParams{ProjectID: &id})
	if err != nil {
		return fmt.Errorf("avance de hitos: %w", err)
	}
	byID := map[uuid.UUID]sqlc.MilestoneProgressRow{}
	for _, pr := range progress {
		byID[pr.MilestoneID] = pr
	}
	for _, m := range ms {
		pr := byID[m.ID]
		line := MilestoneLine{ID: m.ID, Name: m.Name, DueDate: workcore.FromTime(m.DueDate), Status: m.Status,
			Closed: pr.Closed, Total: pr.Total}
		real := 0.0
		if pr.Weight > 0 {
			line.Percent = int(pr.WeightClosed * 100 / pr.Weight)
			real = float64(pr.WeightClosed*100) / float64(pr.Weight)
		}
		mp := plan.Milestones[m.ID]
		line.Figures = schedule.Make(mp.Plan, mp.HasPlan && m.Status == "abierto", real, plan.Settings) // RF-25, RF-34
		out.Milestones = append(out.Milestones, line)
	}
	sort.SliceStable(out.Milestones, func(i, j int) bool {
		a, b := out.Milestones[i].DueDate, out.Milestones[j].DueDate
		if a == nil || b == nil {
			return b == nil && a != nil
		}
		return a.Before(b.Time)
	})
	// The next one neither met nor cancelled (RF-10).
	for _, m := range out.Milestones {
		if m.Status == "abierto" && m.DueDate != nil {
			next := m
			left := days(today, m.DueDate.Time)
			next.DaysLeft = &left
			out.NextMilestone = &next
			break
		}
	}
	return nil
}

func (s *Service) overviewRisks(ctx context.Context, q *sqlc.Queries, row sqlc.Project, today time.Time, plan schedule.ProjectPlan, out *Overview) error {
	items, err := q.ProjectOpenItems(ctx, row.ID)
	if err != nil {
		return fmt.Errorf("tareas abiertas: %w", err)
	}
	var risks []Risk
	rank := map[string]int{"overdue": 0, "blocked": 1, "behind": 2}
	for _, it := range items {
		r := Risk{ID: it.ID, Key: row.Identifier + "-" + strconv.Itoa(int(it.SequenceNumber)), Title: it.Title,
			DueDate: workcore.FromTime(it.DueDate), Assignees: []workcore.Ref{}}
		late := it.DueDate != nil && it.DueDate.Before(today)
		if late {
			out.Overdue++
		}
		if it.Blocked {
			out.Blocked++
		}
		// Behind is at risk or late in the traffic light (spec 009, RF-47).
		f := plan.ByItem[it.ID]
		if f.HasPlan {
			r.Gap, r.Deviation = f.Gap, int(math.Round(f.Gap))
		}
		behind := f.Level == schedule.AtRisk || f.Level == schedule.Late
		if behind {
			r.Level = f.Level
		}
		switch {
		case late:
			r.Reason, r.Days = "overdue", days(*it.DueDate, today)
		case it.Blocked:
			r.Reason = "blocked"
		case behind:
			r.Reason = "behind"
		default:
			continue
		}
		risks = append(risks, r)
	}
	sort.SliceStable(risks, func(i, j int) bool {
		a, b := risks[i], risks[j]
		if rank[a.Reason] != rank[b.Reason] {
			return rank[a.Reason] < rank[b.Reason]
		}
		if a.Days != b.Days {
			return a.Days > b.Days
		}
		return a.Gap < b.Gap
	})
	risks = risks[:min(maxRisks, len(risks))]
	ids := make([]uuid.UUID, len(risks))
	for i, r := range risks {
		ids[i] = r.ID
	}
	people, err := q.ListWorkItemAssignees(ctx, ids)
	if err != nil {
		return fmt.Errorf("asignados: %w", err)
	}
	for i := range risks {
		for _, a := range people {
			if a.WorkItemID == risks[i].ID {
				risks[i].Assignees = append(risks[i].Assignees, workcore.Ref{ID: a.ID, Name: a.Name})
			}
		}
	}
	out.Risks = append(out.Risks, risks...)
	return nil
}

func days(a, b time.Time) int {
	da := time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, time.UTC)
	db := time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, time.UTC)
	return int(math.Round(db.Sub(da).Hours() / 24))
}

// RecordProgress keeps today's progress of every active project, at start and
// then every interval, until ctx ends (RF-12).
func RecordProgress(ctx context.Context, store *db.Store, logger *slog.Logger, interval time.Duration) {
	run := func() {
		now := time.Now().UTC()
		day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		if err := store.RecordDailyProgress(ctx, day); err != nil && ctx.Err() == nil {
			logger.Error("registrar avance diario", "error", err)
		}
	}
	run()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			run()
		}
	}
}

// Column limits of the kanban (RF-66) ----------------------------------------------

// SetColumnLimit sets or clears the limit of one status column.
func (s *Service) SetColumnLimit(ctx context.Context, p *authz.Principal, id, statusID uuid.UUID, limit *int32) (Project, error) {
	if limit != nil && (*limit < 1 || *limit > 999) {
		return Project{}, apperr.ErrValidation.WithDetail("field", "limit")
	}
	var out Project
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		row, err := s.teamChange(ctx, q, p, id)
		if err != nil {
			return err
		}
		if _, err := q.GetStatus(ctx, statusID); err != nil {
			return apperr.ErrValidation.WithDetail("field", "status_id")
		}
		if limit == nil {
			err = q.DeleteColumnLimit(ctx, sqlc.DeleteColumnLimitParams{ProjectID: id, StatusID: statusID})
		} else {
			err = q.SetColumnLimit(ctx, sqlc.SetColumnLimitParams{ProjectID: id, StatusID: statusID, WipLimit: *limit})
		}
		if err != nil {
			return fmt.Errorf("guardar límite: %w", err)
		}
		if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "project.column_limit_changed", TargetType: "project",
			TargetID: id.String(), After: map[string]any{"status_id": statusID, "limit": limit}}); err != nil {
			return err
		}
		out, err = view(ctx, q, p, row)
		return err
	})
	return out, err
}
