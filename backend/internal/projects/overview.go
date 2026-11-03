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
	"openscum/internal/workcore"
)

// Overview is the project at a glance (spec 008, RF-6 to RF-17).
type Overview struct {
	Today string `json:"today"`
	// Weighted by weight, closed ones at 100 % (spec 008, Conceptos).
	Progress  int  `json:"progress"`
	HasPlan   bool `json:"has_plan"` // false: «Sin plazo» (RF-7)
	Expected  int  `json:"expected"`
	Deviation int  `json:"deviation"`

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
}

// Risk is an open work item that is overdue, blocked or behind (RF-15).
type Risk struct {
	ID        uuid.UUID      `json:"id"`
	Key       string         `json:"key"`
	Title     string         `json:"title"`
	Reason    string         `json:"reason"` // overdue, blocked or behind
	Days      int            `json:"days"`   // overdue
	Deviation int            `json:"deviation"`
	DueDate   *workcore.Date `json:"due_date"`
	Assignees []workcore.Ref `json:"assignees"`
}

type DayProgress struct {
	Day      workcore.Date `json:"day"`
	Progress int16         `json:"progress"`
}

const (
	maxRisks     = 10
	maxLoad      = 5
	behindPoints = 10 // as the «retrasada» of spec 007 (RF-13)
)

func (s *Service) Overview(ctx context.Context, p *authz.Principal, id uuid.UUID, today time.Time) (Overview, error) {
	q := s.store.Queries
	row, err := Get(ctx, q, p, id, false)
	if err != nil {
		return Overview{}, err
	}
	out := Overview{Today: today.Format(time.DateOnly), Statuses: []StatusCount{}, Milestones: []MilestoneLine{},
		Risks: []Risk{}, Load: []Member{}, History: []DayProgress{}}
	progress, err := q.ProjectProgress(ctx, id)
	if err != nil {
		return out, fmt.Errorf("avance: %w", err)
	}
	out.Progress = int(progress)
	if row.StartDate != nil && row.EndDate != nil && !row.EndDate.Before(*row.StartDate) {
		out.HasPlan = true
		out.Expected = expected(*row.StartDate, *row.EndDate, today)
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

	if err := s.overviewMilestones(ctx, q, id, today, &out); err != nil {
		return out, err
	}
	if err := s.overviewRisks(ctx, q, row, today, &out); err != nil {
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
	return out, nil
}

func (s *Service) overviewMilestones(ctx context.Context, q *sqlc.Queries, id uuid.UUID, today time.Time, out *Overview) error {
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
		if pr.Weight > 0 {
			line.Percent = int(pr.WeightClosed * 100 / pr.Weight)
		}
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

func (s *Service) overviewRisks(ctx context.Context, q *sqlc.Queries, row sqlc.Project, today time.Time, out *Overview) error {
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
		ratio := 0
		if it.WeightedDoneRatio != nil {
			ratio = int(*it.WeightedDoneRatio)
		}
		if it.StartDate != nil && it.DueDate != nil && !it.DueDate.Before(*it.StartDate) {
			r.Deviation = ratio - expected(*it.StartDate, *it.DueDate, today)
		}
		switch {
		case late:
			r.Reason, r.Days = "overdue", days(*it.DueDate, today)
		case it.Blocked:
			r.Reason = "blocked"
		case r.Deviation < -behindPoints:
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
		return a.Deviation < b.Deviation
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

// expected is the share of a span of dates already gone, both ends included.
func expected(start, end, today time.Time) int {
	total := days(start, end) + 1
	gone := min(max(days(start, today)+1, 0), total)
	return int(math.Round(100 * float64(gone) / float64(total)))
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
