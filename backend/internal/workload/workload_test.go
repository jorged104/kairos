package workload

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"openscum/internal/db/sqlc"
	"openscum/internal/workcore"
)

func date(s string) *time.Time {
	t, _ := time.Parse(time.DateOnly, s)
	return &t
}

func TestBuild(t *testing.T) {
	ana, bea, eva := workcore.Ref{ID: uuid.New(), Name: "Ana"}, workcore.Ref{ID: uuid.New(), Name: "Bea"}, workcore.Ref{ID: uuid.New(), Name: "Eva"}
	items := []sqlc.PersonScheduleItemsRow{
		// The start after the due date: the due day only.
		{UserID: ana.ID, Identifier: "P", SequenceNumber: 1, StartDate: date("2026-03-30"), DueDate: date("2026-03-20"), Weight: 2},
		{UserID: ana.ID, Identifier: "P", SequenceNumber: 2, Weight: 3},
		// Overdue: busy until today.
		{UserID: bea.ID, Identifier: "P", SequenceNumber: 3, StartDate: date("2026-02-01"), DueDate: date("2026-02-10"), Weight: 1},
		{UserID: uuid.New(), Identifier: "P", SequenceNumber: 4, DueDate: date("2026-05-01")},
	}
	rows := Build([]workcore.Ref{ana, bea, eva}, items, time.Date(2026, 3, 10, 15, 0, 0, 0, time.UTC), nil)
	if rows[0].User != eva || rows[0].FreeFrom != nil || len(rows[0].Bars) != 0 {
		t.Fatalf("libre primero = %+v", rows[0])
	}
	b := rows[1]
	if b.User != bea || b.FreeFrom.Format(time.DateOnly) != "2026-03-11" || !b.Bars[0].Overdue || b.Bars[0].Start.Format(time.DateOnly) != "2026-02-01" {
		t.Fatalf("Bea = %+v", b)
	}
	a := rows[2]
	if a.User != ana || a.FreeFrom.Format(time.DateOnly) != "2026-03-21" || a.Open != 2 || a.Undated != 1 || a.Weight != 5 ||
		a.Bars[0].Start.Format(time.DateOnly) != "2026-03-20" || a.Bars[0].Key != "P-1" {
		t.Fatalf("Ana = %+v", a)
	}
}
