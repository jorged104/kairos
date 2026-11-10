// Package workload builds the schedule by person: the bars of the open work
// items of each person and the day they are free (spec 021).
package workload

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"openscum/internal/db/sqlc"
	"openscum/internal/workcore"
)

type Project struct {
	ID         uuid.UUID `json:"id"`
	Identifier string    `json:"identifier"`
	Name       string    `json:"name"`
}

// Bar is an open work item with a due date. Without a start, or with the start
// after the due date, it takes the due day only.
type Bar struct {
	ID        uuid.UUID     `json:"id"`
	Key       string        `json:"key"`
	Title     string        `json:"title"`
	Project   Project       `json:"project"`
	Start     workcore.Date `json:"start"`
	End       workcore.Date `json:"end"`
	DoneRatio int16         `json:"done_ratio"`
	Overdue   bool          `json:"overdue"` // RF-5
	Loose     bool          `json:"loose"`   // without a project (spec 022, RF-36)
}

// Row is one person: their bars, the open work items without a due date and
// the day they are free, null when they are already (RF-1 to RF-4).
type Row struct {
	User     workcore.Ref   `json:"user"`
	FreeFrom *workcore.Date `json:"free_from"`
	Open     int            `json:"open"`
	Weight   int64          `json:"weight"`
	Undated  int            `json:"undated"`
	Bars     []Bar          `json:"bars"`
}

func day(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC) }

// Build makes a row for each person, whoever is free sooner first (RF-7).
func Build(people []workcore.Ref, items []sqlc.PersonScheduleItemsRow, today time.Time) []Row {
	today = day(today)
	at := map[uuid.UUID]int{}
	out := make([]Row, len(people))
	for i, u := range people {
		at[u.ID] = i
		out[i] = Row{User: u, Bars: []Bar{}}
	}
	last := make([]time.Time, len(people))
	for _, it := range items {
		i, ok := at[it.UserID]
		if !ok {
			continue
		}
		r := &out[i]
		r.Open++
		r.Weight += int64(it.Weight)
		if it.DueDate == nil {
			r.Undated++
			continue
		}
		end := day(*it.DueDate)
		start := end
		if it.StartDate != nil && !day(*it.StartDate).After(end) {
			start = day(*it.StartDate)
		}
		b := Bar{
			ID: it.ID, Key: fmt.Sprintf("%s-%d", it.Identifier, it.SequenceNumber), Title: it.Title,
			Project: Project{ID: it.ProjectID, Identifier: it.Identifier, Name: it.ProjectName},
			Start:   workcore.Date{Time: start}, End: workcore.Date{Time: end}, DoneRatio: it.DoneRatio,
			Overdue: end.Before(today), Loose: it.IsInbox,
		}
		r.Bars = append(r.Bars, b)
		// An overdue item keeps the person busy until today.
		busy := end
		if b.Overdue {
			busy = today
		}
		if busy.After(last[i]) {
			last[i] = busy
		}
	}
	for i := range out {
		if !last[i].IsZero() {
			out[i].FreeFrom = &workcore.Date{Time: last[i].AddDate(0, 0, 1)}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].FreeFrom, out[j].FreeFrom
		switch {
		case a == nil && b != nil:
			return true
		case a != nil && b == nil:
			return false
		case a != nil && !a.Equal(b.Time):
			return a.Before(b.Time)
		}
		return out[i].User.Name < out[j].User.Name
	})
	return out
}
