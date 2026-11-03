package workitems

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"

	"openscum/internal/authz"
	"openscum/internal/db/sqlc"
	"openscum/internal/projects"
	"openscum/internal/schedule"
	"openscum/internal/workcore"
)

// Metrics of a work item (spec 007, RF-9 to RF-35). Durations are in days with
// one decimal; a nil one is «Sin datos» (RF-35) or «—» (RF-24).
type Metrics struct {
	Today        string       `json:"today"`
	Schedule     *Schedule    `json:"schedule"` // nil while closed (RF-15)
	Contribution Contribution `json:"contribution"`
	Times        Times        `json:"times"`
	Due          Due          `json:"due"`
	Reopens      *int         `json:"reopens"`
	Blocked      Blocked      `json:"blocked"`
}

// Schedule is the plan, the gap and the level of the work item against its
// baseline (spec 009, RF-11, RF-12, RF-43). Expected, actual and deviation
// are the same rounded, for the earlier API.
type Schedule struct {
	HasPlan   bool           `json:"has_plan"`
	Expected  int            `json:"expected"`
	Actual    int            `json:"actual"`
	Deviation int            `json:"deviation"`
	State     schedule.Level `json:"state"` // ahead, on_track, at_risk or late
	Plan      float64        `json:"plan"`
	Real      float64        `json:"real"`
	Gap       float64        `json:"gap"`

	InBaseline bool           `json:"in_baseline"`
	BaseStart  *workcore.Date `json:"base_start"`
	BaseDue    *workcore.Date `json:"base_due"`
	Changed    bool           `json:"changed"`
	ShiftDays  *int           `json:"shift_days"`
}

// Contribution of the subtree of a work item to its parent and project (RF-16 to RF-20).
type Contribution struct {
	Tracking bool   `json:"tracking"`
	Parent   *Share `json:"parent"` // nil without a parent
	Project  Share  `json:"project"`
}

type Share struct {
	NoWeight bool    `json:"no_weight"`
	Percent  float64 `json:"percent"`
	Points   float64 `json:"points"`
}

// Times: lead and cycle time once closed, age and time in progress while open (RF-21 to RF-25).
type Times struct {
	Closed         bool     `json:"closed"`
	LeadDays       *float64 `json:"lead_days"`
	CycleDays      *float64 `json:"cycle_days"`
	AgeDays        *float64 `json:"age_days"`
	InProgressDays *float64 `json:"in_progress_days"`
	InStatusDays   *float64 `json:"in_status_days"`
	Missing        bool     `json:"missing"`
}

// Due is none, pending, overdue, on_time or late, with its days (RF-26 to RF-29).
type Due struct {
	State string `json:"state"`
	Days  int    `json:"days"`
}

type Blocked struct {
	Now       bool      `json:"now"`
	Blockers  []Blocker `json:"blockers"`
	TotalDays *float64  `json:"total_days"`
}

// Blocker is hidden when the caller can't see it (RF-32).
type Blocker struct {
	Hidden bool       `json:"hidden"`
	ID     *uuid.UUID `json:"id,omitempty"`
	Key    string     `json:"key,omitempty"`
	Title  string     `json:"title,omitempty"`
}

// Metrics computes the metrics of a work item; today is the caller's date.
func (s *Service) Metrics(ctx context.Context, p *authz.Principal, id uuid.UUID, today time.Time) (Metrics, error) {
	q := s.store.Queries
	row, _, err := Load(ctx, q, p, id, false)
	if err != nil {
		return Metrics{}, err
	}
	closedRows, err := q.ClosedStatusIDs(ctx)
	if err != nil {
		return Metrics{}, fmt.Errorf("leer estados cerrados: %w", err)
	}
	isClosed := func(id uuid.UUID) bool { return slices.Contains(closedRows, id) }
	now := time.Now().UTC()
	closed := isClosed(row.StatusID)
	ratio := 0
	if row.WeightedDoneRatio != nil {
		ratio = int(*row.WeightedDoneRatio)
	}
	if closed {
		ratio = 100
	}

	out := Metrics{Today: today.Format(time.DateOnly)}
	if !closed {
		if out.Schedule, err = planOf(ctx, q, row, today); err != nil {
			return out, err
		}
	}
	if out.Contribution, err = contribution(ctx, q, row, ratio); err != nil {
		return out, err
	}
	out.Due = due(row.DueDate, row.ClosedAt, closed, today)

	audits, err := q.WorkItemAudit(ctx, []string{row.ID.String()})
	if err != nil {
		return out, fmt.Errorf("leer historial: %w", err)
	}
	own := timeline(row.CreatedAt, row.StatusID, nil, audits)
	out.Times = times(own, row.ClosedAt, closed, now, isClosed)
	if !own.missing {
		n := own.reopens(isClosed)
		out.Reopens = &n
	}
	out.Blocked, err = s.blocked(ctx, q, p, row, own, now, isClosed)
	return out, err
}

// planOf is the plan of the work item over its subtree, from the plan of
// its project (spec 009, RF-19, RF-43).
func planOf(ctx context.Context, q *sqlc.Queries, row sqlc.WorkItem, today time.Time) (*Schedule, error) {
	plan, err := schedule.LoadProject(ctx, q, row.ProjectID, today)
	if err != nil {
		return nil, err
	}
	for _, r := range plan.Rows {
		if r.ID != row.ID {
			continue
		}
		it := projects.ItemScheduleOf(plan, r)
		f := it.Figures
		out := &Schedule{HasPlan: f.HasPlan, Actual: int(math.Round(f.Real)), Real: f.Real, InBaseline: it.InBaseline,
			BaseStart: it.BaseStart, BaseDue: it.BaseDue, Changed: it.Changed, ShiftDays: it.ShiftDays}
		if f.HasPlan {
			out.Plan, out.Gap, out.State = f.Plan, f.Gap, f.Level
			out.Expected = int(math.Round(f.Plan))
			out.Deviation = out.Actual - out.Expected
		}
		return out, nil
	}
	return &Schedule{}, nil
}

func contribution(ctx context.Context, q *sqlc.Queries, row sqlc.WorkItem, ratio int) (Contribution, error) {
	out := Contribution{Tracking: row.AggWeight == 0}
	share := func(total int64) Share {
		if total == 0 {
			return Share{NoWeight: true}
		}
		pct := 100 * float64(row.AggWeight) / float64(total)
		return Share{Percent: round1(pct), Points: round1(pct * float64(ratio) / 100)}
	}
	if row.ParentID != nil {
		parent, err := q.GetWorkItem(ctx, *row.ParentID)
		if err != nil {
			return out, fmt.Errorf("leer padre: %w", err)
		}
		s := share(parent.AggWeight)
		out.Parent = &s
	}
	total, err := q.ProjectWeight(ctx, row.ProjectID)
	if err != nil {
		return out, fmt.Errorf("peso del proyecto: %w", err)
	}
	out.Project = share(total)
	return out, nil
}

func due(dueDate, closedAt *time.Time, closed bool, today time.Time) Due {
	if dueDate == nil {
		return Due{State: "none"}
	}
	if closed && closedAt != nil {
		if late := days(*dueDate, *closedAt); late > 0 {
			return Due{State: "late", Days: late}
		}
		return Due{State: "on_time"}
	}
	if left := days(today, *dueDate); left >= 0 {
		return Due{State: "pending", Days: left}
	}
	return Due{State: "overdue", Days: days(*dueDate, today)}
}

func times(t statusTimeline, closedAt *time.Time, closed bool, now time.Time, isClosed func(uuid.UUID) bool) Times {
	out := Times{Closed: closed, Missing: t.missing}
	started := t.firstStart(isClosed)
	if closed {
		if closedAt != nil {
			out.LeadDays = span(t.created, *closedAt)
			if started != nil && !started.After(*closedAt) {
				out.CycleDays = span(*started, *closedAt)
			}
		}
	} else {
		out.AgeDays = span(t.created, now)
		if started != nil {
			out.InProgressDays = span(*started, now)
		}
	}
	if !t.missing {
		out.InStatusDays = span(t.lastChange(), now)
	} else {
		out.CycleDays, out.InProgressDays = nil, nil
	}
	return out
}

// blocked lists the open blockers and adds up the time the work item was
// blocked while open (RF-31 to RF-33).
func (s *Service) blocked(ctx context.Context, q *sqlc.Queries, p *authz.Principal, row sqlc.WorkItem, own statusTimeline,
	now time.Time, isClosed func(uuid.UUID) bool) (Blocked, error) {
	out := Blocked{Blockers: []Blocker{}}
	open, err := q.OpenBlockers(ctx, row.ID)
	if err != nil {
		return out, fmt.Errorf("leer bloqueos: %w", err)
	}
	out.Now = len(open) > 0
	if len(open) > 0 {
		rows, err := q.WorkItemRefs(ctx, open)
		if err != nil {
			return out, fmt.Errorf("leer referencias: %w", err)
		}
		for _, r := range rows {
			b := Blocker{Hidden: true}
			if project, err := projects.Get(ctx, q, p, r.ProjectID, false); err == nil && projects.Visible(p, project) {
				id := r.ID
				b = Blocker{ID: &id, Key: Key(r.Identifier, r.SequenceNumber), Title: r.Title}
			}
			out.Blockers = append(out.Blockers, b)
		}
	}

	// When each blocker blocked this work item: from the audit, or since the
	// dependency was made when it wasn't audited.
	links := map[uuid.UUID][]interval{}
	depAudit, err := q.BlockingDependencyAudit(ctx, row.ID.String())
	if err != nil {
		return out, fmt.Errorf("leer historial de bloqueos: %w", err)
	}
	for _, a := range depAudit {
		from, err := uuid.Parse(a.TargetID)
		if err != nil {
			continue
		}
		list := links[from]
		if a.Action == "work_item.dependency_added" {
			links[from] = append(list, interval{a.OccurredAt, now})
		} else if n := len(list); n > 0 && list[n-1].end.Equal(now) {
			list[n-1].end = a.OccurredAt
		}
	}
	current, err := q.CurrentBlockingDependencies(ctx, row.ID)
	if err != nil {
		return out, fmt.Errorf("leer bloqueos: %w", err)
	}
	for _, d := range current {
		list := links[d.FromID]
		if n := len(list); n == 0 || !list[n-1].end.Equal(now) {
			links[d.FromID] = append(list, interval{d.CreatedAt, now})
		}
	}
	if len(links) == 0 {
		zero := 0.0
		out.TotalDays = &zero
		return out, nil
	}
	if own.missing {
		return out, nil
	}

	blockers := make([]uuid.UUID, 0, len(links))
	ids := make([]string, 0, len(links))
	for id := range links {
		blockers = append(blockers, id)
		ids = append(ids, id.String())
	}
	audits, err := q.WorkItemAudit(ctx, ids)
	if err != nil {
		return out, fmt.Errorf("leer historial de bloqueantes: %w", err)
	}
	byItem := map[string][]sqlc.WorkItemAuditRow{}
	for _, a := range audits {
		byItem[a.TargetID] = append(byItem[a.TargetID], a)
	}
	alive := map[uuid.UUID]sqlc.WorkItem{}
	for _, id := range blockers {
		if b, err := q.GetWorkItem(ctx, id); err == nil {
			alive[id] = b
		}
	}

	var blockedSpans []interval
	for _, id := range blockers {
		var t statusTimeline
		if b, ok := alive[id]; ok {
			t = timeline(b.CreatedAt, b.StatusID, nil, byItem[id.String()])
		} else {
			t = deletedTimeline(byItem[id.String()])
		}
		if t.missing {
			return out, nil // «Sin datos» (RF-35)
		}
		blockedSpans = append(blockedSpans, intersect(links[id], t.open(isClosed, now))...)
	}
	total := intersect(union(blockedSpans), own.open(isClosed, now))
	var d time.Duration
	for _, i := range total {
		d += i.end.Sub(i.start)
	}
	out.TotalDays = span(time.Time{}, time.Time{}.Add(d))
	return out, nil
}

// statusTimeline is the status of a work item over time, rebuilt from the
// audit: every change keeps the status before and after.
type statusTimeline struct {
	created time.Time
	changes []statusStep // the first one is the initial status, at created
	end     *time.Time   // deleted
	missing bool
}

type statusStep struct {
	at       time.Time
	from, to uuid.UUID
}

func statusesOf(raw json.RawMessage) (uuid.UUID, bool) {
	var v struct {
		StatusID *uuid.UUID `json:"status_id"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &v) != nil || v.StatusID == nil {
		return uuid.Nil, false
	}
	return *v.StatusID, true
}

func timeline(created time.Time, current uuid.UUID, end *time.Time, audits []sqlc.WorkItemAuditRow) statusTimeline {
	t := statusTimeline{created: created, end: end}
	initial, known := current, false
	for _, a := range audits {
		switch a.Action {
		case "work_item.created":
			if s, ok := statusesOf(a.After); ok && !known {
				initial, known = s, true
			}
		case "work_item.updated":
			from, ok1 := statusesOf(a.Before)
			to, ok2 := statusesOf(a.After)
			if !ok1 || !ok2 {
				t.missing = true
				continue
			}
			if !known {
				initial, known = from, true
			}
			if from != to {
				t.changes = append(t.changes, statusStep{at: a.OccurredAt, from: from, to: to})
			}
		}
	}
	t.changes = append([]statusStep{{at: created, to: initial}}, t.changes...)
	return t
}

// deletedTimeline rebuilds a blocker that no longer exists from its audit.
func deletedTimeline(audits []sqlc.WorkItemAuditRow) statusTimeline {
	var created, end *time.Time
	current := uuid.Nil
	for _, a := range audits {
		switch a.Action {
		case "work_item.created":
			at := a.OccurredAt
			created = &at
		case "work_item.deleted":
			at := a.OccurredAt
			end = &at
			if s, ok := statusesOf(a.Before); ok {
				current = s
			}
		}
	}
	if created == nil || end == nil || current == uuid.Nil {
		return statusTimeline{missing: true}
	}
	return timeline(*created, current, end, audits)
}

// open lists when the work item was in an open status.
func (t statusTimeline) open(isClosed func(uuid.UUID) bool, now time.Time) []interval {
	stop := now
	if t.end != nil {
		stop = *t.end
	}
	var out []interval
	for i, c := range t.changes {
		until := stop
		if i+1 < len(t.changes) {
			until = t.changes[i+1].at
		}
		if !isClosed(c.to) && until.After(c.at) {
			out = append(out, interval{c.at, until})
		}
	}
	return out
}

// firstStart is when it first went to an open status other than the initial one (RF-24).
func (t statusTimeline) firstStart(isClosed func(uuid.UUID) bool) *time.Time {
	initial := t.changes[0].to
	for _, c := range t.changes[1:] {
		if c.to != initial && !isClosed(c.to) {
			at := c.at
			return &at
		}
	}
	return nil
}

func (t statusTimeline) lastChange() time.Time { return t.changes[len(t.changes)-1].at }

// reopens counts the changes from a closed status to an open one (RF-30).
func (t statusTimeline) reopens(isClosed func(uuid.UUID) bool) int {
	n := 0
	for _, c := range t.changes[1:] {
		if isClosed(c.from) && !isClosed(c.to) {
			n++
		}
	}
	return n
}

type interval struct{ start, end time.Time }

func union(in []interval) []interval {
	if len(in) == 0 {
		return nil
	}
	sort.Slice(in, func(i, j int) bool { return in[i].start.Before(in[j].start) })
	out := []interval{in[0]}
	for _, i := range in[1:] {
		last := &out[len(out)-1]
		if !i.start.After(last.end) {
			if i.end.After(last.end) {
				last.end = i.end
			}
			continue
		}
		out = append(out, i)
	}
	return out
}

func intersect(a, b []interval) []interval {
	var out []interval
	for _, x := range union(a) {
		for _, y := range union(b) {
			start, end := x.start, x.end
			if y.start.After(start) {
				start = y.start
			}
			if y.end.Before(end) {
				end = y.end
			}
			if end.After(start) {
				out = append(out, interval{start, end})
			}
		}
	}
	return out
}

// days counts the calendar days from a to b, by their dates.
func days(a, b time.Time) int {
	da := time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, time.UTC)
	db := time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, time.UTC)
	return int(math.Round(db.Sub(da).Hours() / 24))
}

func span(a, b time.Time) *float64 {
	d := round1(b.Sub(a).Hours() / 24)
	return &d
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
