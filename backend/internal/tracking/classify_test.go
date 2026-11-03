package tracking

import (
	"testing"
	"time"

	"openscum/internal/schedule"
)

func day(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

func ptr(s string) *time.Time {
	t := day(s)
	return &t
}

func TestState(t *testing.T) {
	today := day("2026-03-10")
	plan := func(l schedule.Level) schedule.Figures { return schedule.Figures{HasPlan: true, Level: l} }
	cases := []struct {
		name     string
		status   string
		due      *time.Time
		barStart *time.Time
		f        schedule.Figures
		want     string
	}{
		{"cumplido aunque vencido", "cumplido", ptr("2026-03-01"), nil, plan(schedule.Late), StateDone},       // RF-20
		{"fecha objetivo pasada", "abierto", ptr("2026-03-09"), nil, plan(schedule.OnTrack), StateLate},       // RF-21
		{"semáforo atrasado", "abierto", ptr("2026-04-01"), nil, plan(schedule.Late), StateLate},              // RF-21
		{"en riesgo", "abierto", nil, ptr("2026-03-01"), plan(schedule.AtRisk), StateAtRisk},                  // RF-22
		{"futuro", "abierto", ptr("2026-05-01"), ptr("2026-03-11"), plan(schedule.OnTrack), StateFuture},      // RF-23
		{"sin datos", "abierto", ptr("2026-05-01"), nil, schedule.Figures{}, StateNoData},                     // RF-24
		{"en ejecución", "abierto", ptr("2026-03-10"), ptr("2026-03-01"), plan(schedule.Ahead), StateRunning}, // RF-25
	}
	for _, c := range cases {
		if got := State(c.status, c.due, c.barStart, c.f, today); got != c.want {
			t.Errorf("%s = %s, want %s", c.name, got, c.want)
		}
	}
}

func TestBarAndReal(t *testing.T) {
	items := []schedule.Item{
		{Weight: 1, Start: ptr("2026-03-05"), Due: ptr("2026-03-10"), Closed: true},
		{Weight: 3, Due: ptr("2026-03-02")}, // no start: its end
		{Weight: 1},                         // no dates
	}
	start, end := Bar(items)
	if start == nil || !start.Equal(day("2026-03-02")) || !end.Equal(day("2026-03-10")) {
		t.Fatalf("bar = %v → %v", start, end)
	}
	if r := MilestoneReal(items); r != 20 {
		t.Fatalf("real = %v", r)
	}
}

func TestSplit(t *testing.T) {
	today := day("2026-03-10")
	leaf := func(title string, start, due, closed *time.Time, isClosed, initial bool) LeafItem {
		return LeafItem{Task: Task{Title: title}, Start: start, Due: due, Closed: closed, IsClosed: isClosed, Initial: initial}
	}
	cols := Split([]LeafItem{
		leaf("vencida poco", nil, ptr("2026-03-08"), nil, false, true),
		leaf("vencida mucho", nil, ptr("2026-02-01"), nil, false, true),
		leaf("empezada por fecha", ptr("2026-03-10"), ptr("2026-03-20"), nil, false, true),
		leaf("empezada por estado", ptr("2026-04-01"), nil, nil, false, false),
		leaf("sin fechas sin empezar", nil, nil, nil, false, true), // nowhere
		leaf("próxima", ptr("2026-03-24"), ptr("2026-03-30"), nil, false, true),
		leaf("demasiado lejos", ptr("2026-03-25"), nil, nil, false, true),
		leaf("cerrada hace poco", nil, nil, ptr("2026-02-08"), true, false),
		leaf("cerrada hace mucho", nil, nil, ptr("2026-02-07"), true, false),
	}, today)
	titles := func(c Column) (out []string) {
		for _, it := range c.Items {
			out = append(out, it.Title)
		}
		return out
	}
	if got := titles(cols.Late); len(got) != 2 || got[0] != "vencida mucho" || cols.Late.Items[0].LateDays != 37 {
		t.Fatalf("late = %v / %+v", got, cols.Late.Items)
	}
	if got := titles(cols.InProgress); len(got) != 2 || got[0] != "empezada por fecha" {
		t.Fatalf("in progress = %v", got)
	}
	if got := titles(cols.Upcoming); len(got) != 1 || got[0] != "próxima" {
		t.Fatalf("upcoming = %v", got)
	}
	if got := titles(cols.Done); len(got) != 1 || got[0] != "cerrada hace poco" {
		t.Fatalf("done = %v", got)
	}
}
