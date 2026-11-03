package schedule

import (
	"testing"
	"time"

	"github.com/google/uuid"
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

func TestFraction(t *testing.T) {
	cases := []struct {
		name       string
		start, due *time.Time
		today      string
		want       float64
		ok         bool
	}{
		{"sin fin", ptr("2026-03-01"), nil, "2026-03-05", 0, false},                       // RF-18
		{"antes del inicio", ptr("2026-03-01"), ptr("2026-03-10"), "2026-02-28", 0, true}, // RF-14
		{"primer día", ptr("2026-03-01"), ptr("2026-03-10"), "2026-03-01", 0.1, true},     // RF-16
		{"a mitad", ptr("2026-03-01"), ptr("2026-03-10"), "2026-03-05", 0.5, true},        // RF-16
		{"el día del fin", ptr("2026-03-01"), ptr("2026-03-10"), "2026-03-10", 1, true},   // RF-15
		{"después del fin", ptr("2026-03-01"), ptr("2026-03-10"), "2026-04-01", 1, true},  // RF-15
		{"solo fin, antes", nil, ptr("2026-03-10"), "2026-03-09", 0, true},                // RF-17
		{"solo fin, ese día", nil, ptr("2026-03-10"), "2026-03-10", 1, true},              // RF-17
		{"un solo día", ptr("2026-03-10"), ptr("2026-03-10"), "2026-03-10", 1, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := Fraction(c.start, c.due, day(c.today))
			if ok != c.ok || got != c.want {
				t.Fatalf("Fraction = %v, %v; want %v, %v", got, ok, c.want, c.ok)
			}
		})
	}
}

func TestPlanAndReal(t *testing.T) {
	today := day("2026-03-05")
	items := []Item{
		{Weight: 3, Ratio: 100, Start: ptr("2026-03-01"), Due: ptr("2026-03-02")}, // 1
		{Weight: 1, Ratio: 0, Start: ptr("2026-03-01"), Due: ptr("2026-03-10")},   // 0.5
		{Weight: 5, Ratio: 50},                        // no plan
		{Weight: 0, Ratio: 0, Due: ptr("2026-03-01")}, // weight 0
	}
	plan, ok := Plan(items, today)
	if !ok || plan != 100*3.5/4 {
		t.Fatalf("plan = %v, %v", plan, ok)
	}
	if got := Real(items); got != 100*(300.0+250)/900 {
		t.Fatalf("real = %v", got)
	}
	if _, ok := Plan([]Item{{Weight: 2}}, today); ok {
		t.Fatal("no planned end must give no plan")
	}
	// All weigh 0: the simple mean (spec 004, RF-153).
	zero := []Item{{Due: ptr("2026-03-01")}, {Due: ptr("2026-03-20")}}
	if plan, _ := Plan(zero, today); plan != 50 {
		t.Fatalf("plan with weight 0 = %v", plan)
	}
}

func TestSubtreePlans(t *testing.T) {
	today := day("2026-03-05")
	root, child, other := uuid.New(), uuid.New(), uuid.New()
	items := []Item{
		{ID: root, Weight: 1},
		{ID: child, ParentID: &root, Weight: 1, Due: ptr("2026-03-01")},
		{ID: other, ParentID: &root, Weight: 3, Due: ptr("2026-03-20")},
	}
	got := SubtreePlans(items, today)
	if f := got[root]; !f.HasPlan || f.Plan != 25 {
		t.Fatalf("root = %+v", f)
	}
	if f := got[child]; f.Plan != 100 {
		t.Fatalf("child = %+v", f)
	}
}

func TestMakeAndClassify(t *testing.T) {
	s := DefaultSettings()
	cases := []struct {
		plan, real float64
		want       Level
	}{
		{50, 55, Ahead},     // +5: the upper bound is included (RF-28)
		{50, 54.9, OnTrack}, // RF-29
		{50, 45, OnTrack},   // −5 is still on track
		{50, 44.9, AtRisk},  // RF-30
		{50, 35, AtRisk},    // −15 is still at risk
		{50, 34.9, Late},    // RF-31
	}
	for _, c := range cases {
		if f := Make(c.plan, true, c.real, s); f.Level != c.want {
			t.Errorf("gap %v = %s, want %s", f.Gap, f.Level, c.want)
		}
	}
	if f := Make(0, false, 40, s); f.HasPlan || f.Level != "" || f.Real != 40 {
		t.Fatalf("without plan = %+v", f)
	}
}
