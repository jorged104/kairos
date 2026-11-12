package projects

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/authz"
	"openscum/internal/db/sqlc"
	"openscum/internal/schedule"
	"openscum/internal/workcore"
)

// Kinds of change of a work item between two sides (spec 024, Conceptos).
const (
	ChangeAdded     = "added"
	ChangeRemoved   = "removed"
	ChangeMoved     = "moved"
	ChangeUnchanged = "unchanged"
)

// Today is the side made of the current dates of the work items.
const Today = "today"

// Comparison is what changed between two versions of the baseline of a
// project, or between one and today.
type Comparison struct {
	Before   Baseline       `json:"before"`
	After    *Baseline      `json:"after"` // null: today
	Versions []Baseline     `json:"versions"`
	Summary  CompareSummary `json:"summary"`
	Items    []CompareItem  `json:"items"`
}

// CompareSummary counts the work items by kind of change and shifts the end
// of the project (RF-9 to RF-11).
type CompareSummary struct {
	Added     int            `json:"added"`
	Removed   int            `json:"removed"`
	Moved     int            `json:"moved"`
	Unchanged int            `json:"unchanged"`
	BeforeEnd *workcore.Date `json:"before_end"`
	AfterEnd  *workcore.Date `json:"after_end"`
	EndShift  *int           `json:"end_shift"`
}

// CompareItem is one work item in the comparison (RF-12 to RF-23). ID is
// null when it cannot be opened: deleted or in a project one does not see.
type CompareItem struct {
	ID          *uuid.UUID     `json:"id"`
	Key         string         `json:"key"`
	Title       string         `json:"title"`
	Change      string         `json:"change"`
	BeforeStart *workcore.Date `json:"before_start"`
	BeforeDue   *workcore.Date `json:"before_due"`
	AfterStart  *workcore.Date `json:"after_start"`
	AfterDue    *workcore.Date `json:"after_due"`
	StartShift  *int           `json:"start_shift"`
	DueShift    *int           `json:"due_shift"`
	Deleted     bool           `json:"deleted"`
	Elsewhere   bool           `json:"elsewhere"`
	Hidden      bool           `json:"hidden"`
}

// side is the dates of each work item on one side.
type side map[uuid.UUID]dates

type dates struct{ start, due *time.Time }

// shown is how a work item is shown, whichever side it comes from.
type shown struct {
	item      CompareItem
	sortKey   string
	sortIndex int
}

// Compare compares two sides of the baseline of a project for whoever sees
// it (RF-27, RF-28). from and to are version numbers or "today"; both empty
// pick the default ones (RF-3, RF-4).
func (s *Service) Compare(ctx context.Context, p *authz.Principal, id uuid.UUID, from, to string) (Comparison, error) {
	q := s.store.Queries
	if _, err := Get(ctx, q, p, id, false); err != nil {
		return Comparison{}, err
	}
	versions, err := q.ListBaselines(ctx, id)
	if err != nil {
		return Comparison{}, fmt.Errorf("líneas base: %w", err)
	}
	if len(versions) == 0 { // RF-1
		return Comparison{}, apperr.ErrNotFound
	}
	out := Comparison{Versions: make([]Baseline, len(versions)), Items: []CompareItem{}}
	for i, v := range versions {
		out.Versions[i] = Baseline{Version: v.Version, Reason: v.Reason, CreatedAt: v.CreatedAt, Author: workcore.Ref{ID: v.AuthorID, Name: v.AuthorName}}
	}
	a, b, err := pickSides(from, to, versions)
	if err != nil {
		return Comparison{}, err
	}
	// The older version is «before»; today is always «after» (RF-5).
	if b != 0 && b < a {
		a, b = b, a
	}

	info := map[uuid.UUID]shown{}
	before, err := baselineSide(ctx, q, p, id, a, info)
	if err != nil {
		return out, err
	}
	out.Before = byVersion(out.Versions, a)
	var after side
	if b == 0 {
		after, err = todaySide(ctx, q, id, info)
	} else {
		after, err = baselineSide(ctx, q, p, id, b, info)
		v := byVersion(out.Versions, b)
		out.After = &v
	}
	if err != nil {
		return out, err
	}

	rows := []shown{}
	for wid, sh := range info {
		bd, inBefore := before[wid]
		ad, inAfter := after[wid]
		it := sh.item
		it.BeforeStart, it.BeforeDue = workcore.FromTime(bd.start), workcore.FromTime(bd.due)
		it.AfterStart, it.AfterDue = workcore.FromTime(ad.start), workcore.FromTime(ad.due)
		switch {
		case inBefore && !inAfter:
			it.Change = ChangeRemoved
			out.Summary.Removed++
		case !inBefore && inAfter:
			it.Change = ChangeAdded
			out.Summary.Added++
		default:
			it.StartShift, it.DueShift = shift(bd.start, ad.start), shift(bd.due, ad.due)
			if sameDay(bd.start, ad.start) && sameDay(bd.due, ad.due) {
				it.Change = ChangeUnchanged
				out.Summary.Unchanged++
			} else { // RF-17
				it.Change = ChangeMoved
				out.Summary.Moved++
			}
		}
		sh.item = it
		rows = append(rows, sh)
	}

	out.Summary.BeforeEnd, out.Summary.AfterEnd = lastDue(before), lastDue(after)
	if out.Summary.BeforeEnd != nil && out.Summary.AfterEnd != nil { // RF-11
		d := schedule.Days(out.Summary.BeforeEnd.Time, out.Summary.AfterEnd.Time)
		out.Summary.EndShift = &d
	}

	// Biggest delay of the end first, then the rest by key (RF-15).
	sort.SliceStable(rows, func(i, j int) bool {
		si, sj := rows[i].item.DueShift, rows[j].item.DueShift
		hi, hj := si != nil && *si != 0, sj != nil && *sj != 0
		switch {
		case hi && hj && *si != *sj:
			return *si > *sj
		case hi != hj:
			return hi
		case rows[i].sortKey != rows[j].sortKey:
			return rows[i].sortKey < rows[j].sortKey
		}
		return rows[i].sortIndex < rows[j].sortIndex
	})
	for _, r := range rows {
		out.Items = append(out.Items, r.item)
	}
	return out, nil
}

// pickSides turns the query into version numbers, 0 being today.
func pickSides(from, to string, versions []sqlc.ListBaselinesRow) (int32, int32, error) {
	if from == "" && to == "" {
		if len(versions) == 1 { // RF-4
			return versions[0].Version, 0, nil
		}
		return versions[1].Version, versions[0].Version, nil // RF-3
	}
	a, err := parseSide(from, versions)
	if err != nil {
		return 0, 0, err
	}
	b, err := parseSide(to, versions)
	if err != nil {
		return 0, 0, err
	}
	if a == b { // RF-6
		return 0, 0, apperr.ErrValidation.WithDetail("field", "to")
	}
	if a == 0 { // today goes after
		a, b = b, a
	}
	return a, b, nil
}

func parseSide(v string, versions []sqlc.ListBaselinesRow) (int32, error) {
	if v == Today {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, apperr.ErrValidation.WithDetail("field", "version")
	}
	for _, b := range versions {
		if int(b.Version) == n {
			return b.Version, nil
		}
	}
	return 0, apperr.ErrNotFound
}

func byVersion(vs []Baseline, v int32) Baseline {
	for _, b := range vs {
		if b.Version == v {
			return b
		}
	}
	return Baseline{}
}

// baselineSide loads one version and how each of its work items is shown now
// (RF-19 to RF-23).
func baselineSide(ctx context.Context, q *sqlc.Queries, p *authz.Principal, projectID uuid.UUID, version int32, info map[uuid.UUID]shown) (side, error) {
	b, err := q.GetBaselineByVersion(ctx, sqlc.GetBaselineByVersionParams{ProjectID: projectID, Version: version})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("línea base: %w", err)
	}
	rows, err := q.BaselineCompareItems(ctx, b.ID)
	if err != nil {
		return nil, fmt.Errorf("tareas de la línea base: %w", err)
	}
	out := make(side, len(rows))
	for _, r := range rows {
		out[r.ID] = dates{r.StartDate, r.DueDate}
		if _, ok := info[r.ID]; ok {
			continue
		}
		it := CompareItem{}
		saved := func() {
			it.Key, it.Title = deref(r.ItemKey), deref(r.SavedTitle)
		}
		switch {
		case r.ProjectID == nil: // RF-19
			it.Deleted = true
			saved()
		case *r.ProjectID == projectID:
			it.ID = &r.ID
			it.Key, it.Title = itemKey(*r.Identifier, *r.SequenceNumber), *r.Title
		default:
			it.Elsewhere = true // RF-20
			ref := authz.ProjectRef{ID: *r.ProjectID, TeamID: derefID(r.TeamID), OwnerID: derefID(r.OwnerID)}
			if !r.IsInbox && p.SeesProject(ref) {
				it.ID = &r.ID
				it.Key, it.Title = itemKey(*r.Identifier, *r.SequenceNumber), *r.Title
			} else { // RF-21, RF-22
				it.Hidden = true
				saved()
			}
		}
		info[r.ID] = shown{item: it, sortKey: keySort(it.Key), sortIndex: len(info)}
	}
	return out, nil
}

// todaySide is the current dates of the work items of the project.
func todaySide(ctx context.Context, q *sqlc.Queries, projectID uuid.UUID, info map[uuid.UUID]shown) (side, error) {
	rows, err := q.CurrentCompareItems(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("tareas actuales: %w", err)
	}
	out := make(side, len(rows))
	for _, r := range rows {
		out[r.ID] = dates{r.StartDate, r.DueDate}
		if _, ok := info[r.ID]; ok {
			continue
		}
		id := r.ID
		key := itemKey(r.Identifier, r.SequenceNumber)
		info[r.ID] = shown{item: CompareItem{ID: &id, Key: key, Title: r.Title}, sortKey: keySort(key), sortIndex: len(info)}
	}
	return out, nil
}

func lastDue(s side) *workcore.Date {
	var last *time.Time
	for _, d := range s {
		if d.due != nil && (last == nil || d.due.After(*last)) {
			last = d.due
		}
	}
	return workcore.FromTime(last)
}

// shift is b - a in days, null when either is missing (RF-16).
func shift(a, b *time.Time) *int {
	if a == nil || b == nil {
		return nil
	}
	d := schedule.Days(*a, *b)
	return &d
}

func itemKey(identifier string, seq int32) string {
	return identifier + "-" + strconv.Itoa(int(seq))
}

// keySort orders APP-9 before APP-10; empty keys go last.
func keySort(key string) string {
	i := strings.LastIndex(key, "-")
	if i < 0 {
		return "~" + key
	}
	return fmt.Sprintf("%s-%010s", key[:i], key[i+1:])
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefID(id *uuid.UUID) uuid.UUID {
	if id == nil {
		return uuid.Nil
	}
	return *id
}
