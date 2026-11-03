// Package tracking is the follow-up of several projects: their figures, the
// calendar of their milestones and the work items that need attention
// (spec 011).
package tracking

import (
	"sort"
	"time"

	"github.com/google/uuid"

	"openscum/internal/schedule"
	"openscum/internal/workcore"
)

// Milestone states of the calendar (RF-20 to RF-25).
const (
	StateDone    = "done"
	StateLate    = "late"
	StateAtRisk  = "at_risk"
	StateFuture  = "future"
	StateNoData  = "no_data"
	StateRunning = "running"
	statusMet    = "cumplido"
	columnLimit  = 100
	upcomingDays = 14
	recentDays   = 30
)

// Bar is where a milestone's bar goes: from the earliest planned start of
// its work items to the latest planned end; a missing start counts its end
// (Conceptos).
func Bar(items []schedule.Item) (start, end *time.Time) {
	for _, it := range items {
		if it.Due == nil {
			continue
		}
		s := it.Due
		if it.Start != nil && it.Start.Before(*it.Due) {
			s = it.Start
		}
		if start == nil || s.Before(*start) {
			start = s
		}
		if end == nil || it.Due.After(*end) {
			end = it.Due
		}
	}
	return start, end
}

// MilestoneReal is the weight of the closed work items over the total, as
// the milestone progress of spec 004.
func MilestoneReal(items []schedule.Item) float64 {
	var w, closed float64
	for _, it := range items {
		w += float64(it.Weight)
		if it.Closed {
			closed += float64(it.Weight)
		}
	}
	if w == 0 {
		return 0
	}
	return 100 * closed / w
}

// State classifies a milestone, the first rule that holds (RF-26).
func State(status string, due *time.Time, barStart *time.Time, f schedule.Figures, today time.Time) string {
	switch {
	case status == statusMet:
		return StateDone
	case (due != nil && schedule.Days(*due, today) > 0) || f.Level == schedule.Late:
		return StateLate
	case f.Level == schedule.AtRisk:
		return StateAtRisk
	case barStart != nil && schedule.Days(today, *barStart) > 0:
		return StateFuture
	case !f.HasPlan:
		return StateNoData
	default:
		return StateRunning
	}
}

// Task is a leaf work item in a column (RF-36 to RF-40).
type Task struct {
	ID          uuid.UUID      `json:"id"`
	Key         string         `json:"key"`
	Title       string         `json:"title"`
	ProjectID   uuid.UUID      `json:"project_id"`
	ParentTitle *string        `json:"parent_title"`
	Ratio       int32          `json:"ratio"`
	StartDate   *workcore.Date `json:"start_date"`
	DueDate     *workcore.Date `json:"due_date"`
	ClosedOn    *workcore.Date `json:"closed_on"`
	LateDays    int            `json:"late_days,omitempty"`
}

// Column holds at most columnLimit work items and how many there are (RF-35, RF-41).
type Column struct {
	Total int    `json:"total"`
	Items []Task `json:"items"`
}

type Columns struct {
	Late       Column `json:"late"`
	InProgress Column `json:"in_progress"`
	Done       Column `json:"done"`
	Upcoming   Column `json:"upcoming"`
}

// LeafItem is what the columns need of a leaf work item.
type LeafItem struct {
	Task
	Start, Due, Closed *time.Time
	IsClosed, Initial  bool
}

// Split puts each leaf work item into its column, sorted (RF-31 to RF-41).
func Split(items []LeafItem, today time.Time) Columns {
	var late, running, done, upcoming []LeafItem
	for _, it := range items {
		switch {
		case it.IsClosed:
			if it.Closed != nil && schedule.Days(*it.Closed, today) <= recentDays { // RF-33
				done = append(done, it)
			}
		case it.Due != nil && schedule.Days(*it.Due, today) > 0: // RF-31
			it.LateDays = schedule.Days(*it.Due, today)
			late = append(late, it)
		case (it.Start != nil && schedule.Days(*it.Start, today) >= 0) || !it.Initial: // RF-32
			running = append(running, it)
		case it.Start != nil && schedule.Days(today, *it.Start) <= upcomingDays: // RF-34
			upcoming = append(upcoming, it)
		}
	}
	before := func(a, b *time.Time) bool { // nil last
		if a == nil || b == nil {
			return b == nil && a != nil
		}
		return a.Before(*b)
	}
	sort.SliceStable(late, func(i, j int) bool { return late[i].LateDays > late[j].LateDays })
	sort.SliceStable(running, func(i, j int) bool { return before(running[i].Due, running[j].Due) })
	sort.SliceStable(done, func(i, j int) bool { return before(done[j].Closed, done[i].Closed) })
	sort.SliceStable(upcoming, func(i, j int) bool { return before(upcoming[i].Start, upcoming[j].Start) })
	return Columns{Late: column(late), InProgress: column(running), Done: column(done), Upcoming: column(upcoming)}
}

func column(items []LeafItem) Column {
	c := Column{Total: len(items), Items: []Task{}}
	for _, it := range items[:min(len(items), columnLimit)] {
		c.Items = append(c.Items, it.Task)
	}
	return c
}
