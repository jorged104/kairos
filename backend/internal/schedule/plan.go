// Package schedule computes the plan, the gap and the traffic light of
// projects, milestones and work items (spec 009).
package schedule

import (
	"math"
	"time"

	"github.com/google/uuid"

	"openscum/internal/db/sqlc"
)

// Level is the traffic light of a gap (RF-28 to RF-31).
type Level string

const (
	Ahead   Level = "ahead"
	OnTrack Level = "on_track"
	AtRisk  Level = "at_risk"
	Late    Level = "late"
)

// Item is a work item as the plan sees it: Start and Due are its planned
// dates, the baseline ones if it is in the baseline (RF-9, RF-10).
type Item struct {
	ID         uuid.UUID
	ParentID   *uuid.UUID
	Weight     int
	Ratio      int // own progress, closed = 100 (RF-26)
	Start      *time.Time
	Due        *time.Time
	InBaseline bool
}

// FromRow resolves the planned dates of a work item.
func FromRow(r sqlc.ProjectPlanItemsRow) Item {
	it := Item{ID: r.ID, ParentID: r.ParentID, Weight: int(r.Weight), Ratio: int(r.Ratio), Start: r.StartDate, Due: r.DueDate}
	if r.InBaseline {
		it.InBaseline, it.Start, it.Due = true, r.BaseStart, r.BaseDue
	}
	return it
}

// Figures are the plan, the real progress and the gap of something, with its
// traffic light (RF-35). Without a plan only Real counts (RF-24, RF-25).
type Figures struct {
	HasPlan bool    `json:"has_plan"`
	Plan    float64 `json:"plan"`
	Real    float64 `json:"real"`
	Gap     float64 `json:"gap"`
	Level   Level   `json:"level,omitempty"`
}

// Days counts the calendar days from a to b.
func Days(a, b time.Time) int {
	da := time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, time.UTC)
	db := time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, time.UTC)
	return int(math.Round(db.Sub(da).Hours() / 24))
}

// Fraction is the part of a work item that should be done by today, and
// false when it has no planned end (RF-14 to RF-18).
func Fraction(start, due *time.Time, today time.Time) (float64, bool) {
	if due == nil {
		return 0, false
	}
	if Days(*due, today) >= 0 {
		return 1, true
	}
	if start == nil || Days(*start, *due) < 0 || Days(*start, today) < 0 {
		return 0, true
	}
	return float64(Days(*start, today)+1) / float64(Days(*start, *due)+1), true
}

type sums struct {
	weight, weighted float64 // over the ones with a planned end
	count            int
	fractions        float64
	// Over all, for the real progress.
	allWeight, allWeighted float64
	all                    int
	ratios                 float64
}

func (s *sums) add(it Item, today time.Time) {
	s.allWeight += float64(it.Weight)
	s.allWeighted += float64(it.Weight * it.Ratio)
	s.all++
	s.ratios += float64(it.Ratio)
	f, ok := Fraction(it.Start, it.Due, today)
	if !ok {
		return
	}
	s.weight += float64(it.Weight)
	s.weighted += float64(it.Weight) * f
	s.count++
	s.fractions += f
}

func (s sums) merge(o sums) sums {
	return sums{s.weight + o.weight, s.weighted + o.weighted, s.count + o.count, s.fractions + o.fractions,
		s.allWeight + o.allWeight, s.allWeighted + o.allWeighted, s.all + o.all, s.ratios + o.ratios}
}

// plan is weighted by weight; when all weigh 0 it is the simple mean, as the
// derived progress (spec 004, RF-153).
func (s sums) plan() (float64, bool) {
	switch {
	case s.count == 0:
		return 0, false
	case s.weight > 0:
		return clamp(100 * s.weighted / s.weight), true
	default:
		return clamp(100 * s.fractions / float64(s.count)), true
	}
}

// derived is the real progress of a subtree, as weighted_done_ratio.
func (s sums) derived() float64 {
	switch {
	case s.allWeight > 0:
		return clamp(s.allWeighted / s.allWeight)
	case s.all > 0:
		return clamp(s.ratios / float64(s.all))
	}
	return 0
}

// Plan of a set of work items: a project or a milestone (RF-20 to RF-22).
func Plan(items []Item, today time.Time) (float64, bool) {
	var s sums
	for _, it := range items {
		s.add(it, today)
	}
	return s.plan()
}

// Real progress of a set of work items, weighted, as the project progress
// (spec 008, Conceptos), with one decimal.
func Real(items []Item) float64 {
	var w, wr float64
	for _, it := range items {
		w += float64(it.Weight)
		wr += float64(it.Weight * it.Ratio)
	}
	if w == 0 {
		return 0
	}
	return clamp(wr / w)
}

// SubtreePlans is the plan and the derived progress of each work item over
// itself and its descendants (RF-19), with its level.
func SubtreePlans(items []Item, today time.Time, set Settings) map[uuid.UUID]Figures {
	children := map[uuid.UUID][]int{}
	for i, it := range items {
		if it.ParentID != nil {
			children[*it.ParentID] = append(children[*it.ParentID], i)
		}
	}
	memo := make(map[uuid.UUID]sums, len(items))
	var walk func(i int) sums
	walk = func(i int) sums {
		it := items[i]
		if s, ok := memo[it.ID]; ok {
			return s
		}
		var s sums
		s.add(it, today)
		for _, c := range children[it.ID] {
			s = s.merge(walk(c))
		}
		memo[it.ID] = s
		return s
	}
	out := make(map[uuid.UUID]Figures, len(items))
	for i, it := range items {
		w := walk(i)
		p, ok := w.plan()
		out[it.ID] = Make(p, ok, w.derived(), set)
	}
	return out
}

// Make completes the figures with the real progress, the gap and the level
// (RF-27).
func Make(plan float64, hasPlan bool, real float64, s Settings) Figures {
	f := Figures{HasPlan: hasPlan, Real: round1(real)}
	if !hasPlan {
		return f
	}
	f.Plan = round1(plan)
	f.Gap = round1(f.Real - f.Plan)
	f.Level = s.Classify(f.Gap)
	return f
}

// Classify puts a gap, in points, into its level (RF-28 to RF-31).
func (s Settings) Classify(gap float64) Level {
	const eps = 1e-9
	switch {
	case gap >= s.Upper-eps:
		return Ahead
	case gap >= s.Risk-eps:
		return OnTrack
	case gap >= s.Late-eps:
		return AtRisk
	default:
		return Late
	}
}

func clamp(v float64) float64 { return math.Min(100, math.Max(0, v)) }

func round1(v float64) float64 { return math.Round(v*10) / 10 }
