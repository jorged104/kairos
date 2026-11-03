package apitest

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

type figuresDTO struct {
	HasPlan bool    `json:"has_plan"`
	Plan    float64 `json:"plan"`
	Real    float64 `json:"real"`
	Gap     float64 `json:"gap"`
	Level   string  `json:"level"`
}

type baselineDTO struct {
	Version int32   `json:"version"`
	Reason  *string `json:"reason"`
	Author  struct {
		Name string `json:"name"`
	} `json:"author"`
}

type scheduleDTO struct {
	Baseline *baselineDTO  `json:"baseline"`
	Versions []baselineDTO `json:"versions"`
	Items    []struct {
		ID         uuid.UUID  `json:"id"`
		Figures    figuresDTO `json:"figures"`
		InBaseline bool       `json:"in_baseline"`
		BaseDue    *string    `json:"base_due"`
		Changed    bool       `json:"changed"`
		ShiftDays  *int       `json:"shift_days"`
	} `json:"items"`
}

func (s *workSetup) schedule(t *testing.T, c *Client, project uuid.UUID, today string) scheduleDTO {
	t.Helper()
	var out scheduleDTO
	c.Get("/api/projects/"+project.String()+"/schedule?today="+today).Expect(t, http.StatusOK).JSON(t, &out)
	return out
}

func TestBaseline(t *testing.T) {
	s := newWorkSetup(t)
	p := s.project(t, s.lead, map[string]any{"identifier": "LB", "name": "Línea base"})
	path := "/api/projects/" + p.ID.String() + "/baseline"
	// Weights 3 and 1: on the 6th, 3 × 1 (ended) + 1 × 0.6 = 3.6 / 4 = 90 % (RF-20).
	a := s.item(t, s.member, p.ID, map[string]any{"title": "A", "weight": 3, "due_date": "2026-03-05"})
	b := s.item(t, s.member, p.ID, map[string]any{"title": "B", "weight": 1, "start_date": "2026-03-01", "due_date": "2026-03-10", "done_ratio": 50})
	if o := s.overview(t, s.viewer, p.ID, "2026-03-06"); o.Figures.Plan != 90 || o.Figures.Real != 12.5 || o.Figures.Level != "late" || len(o.PlanSeries) == 0 {
		t.Fatalf("plan without baseline = %+v (RF-9)", o.Figures)
	}

	// Only whoever manages the project (RF-7).
	s.member.Post(path, map[string]any{}).Expect(t, http.StatusForbidden, "forbidden")
	var first baselineDTO
	s.lead.Post(path, map[string]any{}).Expect(t, http.StatusCreated).JSON(t, &first)
	if first.Version != 1 || first.Reason != nil || first.Author.Name != "Laura" {
		t.Fatalf("first = %+v (RF-3)", first)
	}

	// Moving a work item does not move the plan, and it shows the shift (RF-4, RF-12).
	s.patch(s.member, s.get(t, a.ID), map[string]any{"due_date": "2026-03-20", "weight": 3}).Expect(t, http.StatusOK)
	if o := s.overview(t, s.viewer, p.ID, "2026-03-06"); o.Figures.Plan != 90 {
		t.Fatalf("plan after moving = %+v, want 90", o.Figures)
	}
	// A new one counts with its dates, out of the baseline (RF-10, RF-11).
	c := s.item(t, s.member, p.ID, map[string]any{"title": "C", "weight": 4, "due_date": "2026-03-01"})
	sc := s.schedule(t, s.viewer, p.ID, "2026-03-06")
	for _, it := range sc.Items {
		switch it.ID {
		case a.ID:
			if !it.InBaseline || !it.Changed || it.ShiftDays == nil || *it.ShiftDays != 15 || it.BaseDue == nil || *it.BaseDue != "2026-03-05" {
				t.Fatalf("moved = %+v", it)
			}
		case b.ID:
			if !it.InBaseline || it.Changed || it.Figures.Plan != 60 || it.Figures.Level != "at_risk" {
				t.Fatalf("unchanged = %+v (RF-16, RF-19)", it)
			}
		case c.ID:
			if it.InBaseline {
				t.Fatalf("new = %+v", it)
			}
		}
	}
	if o := s.overview(t, s.viewer, p.ID, "2026-03-06"); o.Figures.Plan != 95 {
		t.Fatalf("plan with the new one = %+v, want (3.6 + 4) / 8", o.Figures)
	}

	// Again: the reason is required and the latest version rules (RF-2, RF-4, RF-6).
	s.lead.Post(path, map[string]any{"reason": "  "}).Expect(t, http.StatusBadRequest, "validation_failed")
	s.lead.Post(path, map[string]any{"reason": string(make([]rune, 501))}).Expect(t, http.StatusBadRequest, "validation_failed")
	s.lead.Post(path, map[string]any{"reason": "Cambio de alcance"}).Expect(t, http.StatusCreated)
	sc = s.schedule(t, s.viewer, p.ID, "2026-03-06")
	if sc.Baseline == nil || sc.Baseline.Version != 2 || *sc.Baseline.Reason != "Cambio de alcance" || len(sc.Versions) != 1 || sc.Versions[0].Version != 1 {
		t.Fatalf("versions = %+v / %+v", sc.Baseline, sc.Versions)
	}
	if o := s.overview(t, s.viewer, p.ID, "2026-03-06"); o.Figures.Plan != 57.5 {
		t.Fatalf("plan with version 2 = %+v, want (3 × 0 + 0.6 + 4) / 8", o.Figures)
	}
	if acts := s.env.AuditActions(p.ID.String()); acts[len(acts)-1] != "project.baseline_set" { // RF-49
		t.Fatalf("audit = %v", acts)
	}

	// Closed: no baseline and no plan (RF-8, RF-34).
	s.lead.Post("/api/projects/"+p.ID.String()+"/close", map[string]any{"version": s.getProject(t, p.ID).Version}).Expect(t, http.StatusOK)
	s.lead.Post(path, map[string]any{"reason": "x"}).Expect(t, http.StatusConflict)
	if o := s.overview(t, s.viewer, p.ID, "2026-03-06"); o.HasPlan || o.Figures.Level != "" {
		t.Fatalf("closed = %+v", o.Figures)
	}
}

func TestScheduleMilestonesAndFallback(t *testing.T) {
	s := newWorkSetup(t)
	cerrado := s.env.CatalogID("statuses", "Cerrado")
	p := s.project(t, s.lead, map[string]any{"identifier": "HT", "name": "Hitos", "start_date": "2026-03-01", "end_date": "2026-03-10"})
	// Without planned ends, the project's dates (RF-23); the milestone, «Sin plazo» (RF-25).
	free := s.item(t, s.member, p.ID, map[string]any{"title": "Libre"})
	m := s.milestone(t, p.ID, map[string]any{"name": "Entrega"})
	s.link(t, s.lead, m.ID, free.ID).Expect(t, http.StatusNoContent)
	o := s.overview(t, s.viewer, p.ID, "2026-03-06")
	if !o.Figures.HasPlan || o.Figures.Plan != 60 || o.Milestones[0].Figures.HasPlan {
		t.Fatalf("fallback = %+v, milestone %+v", o.Figures, o.Milestones[0].Figures)
	}
	dated := s.item(t, s.member, p.ID, map[string]any{"title": "Con fecha", "due_date": "2026-03-02"})
	s.link(t, s.lead, m.ID, dated.ID).Expect(t, http.StatusNoContent)
	s.patch(s.admin, s.get(t, dated.ID), map[string]any{"status_id": cerrado, "due_date": "2026-03-02"}).Expect(t, http.StatusOK)
	o = s.overview(t, s.viewer, p.ID, "2026-03-06")
	if f := o.Milestones[0].Figures; !f.HasPlan || f.Plan != 100 || f.Real != 50 || f.Level != "late" { // RF-21, RF-45
		t.Fatalf("milestone = %+v", f)
	}
	// Closed work items show no plan (RF-34).
	for _, it := range s.schedule(t, s.viewer, p.ID, "2026-03-06").Items {
		if it.ID == dated.ID && (it.Figures.HasPlan || it.Figures.Real != 100) {
			t.Fatalf("closed item = %+v", it)
		}
	}
}

func TestScheduleSettings(t *testing.T) {
	s := newWorkSetup(t)
	var cur map[string]any
	s.viewer.Get("/api/schedule-settings").Expect(t, http.StatusOK).JSON(t, &cur)
	if cur["upper"] != 5.0 || cur["risk"] != -5.0 || cur["late"] != -15.0 { // RF-32
		t.Fatalf("defaults = %+v", cur)
	}
	body := func(upper, risk, late float64, version any) map[string]any {
		return map[string]any{"upper": upper, "risk": risk, "late": late, "version": version, "levels": map[string]any{
			"ahead": map[string]any{"name": "Por delante", "color": "violet"}, "on_track": map[string]any{"name": nil, "color": "green"},
			"at_risk": map[string]any{"name": " ", "color": "amber"}, "late": map[string]any{"name": nil, "color": "red"},
		}}
	}
	put := func(c *Client, b map[string]any) *Response { return c.Do(http.MethodPut, "/api/schedule-settings", b) }
	put(s.lead, body(10, 0, -10, 1)).Expect(t, http.StatusForbidden, "forbidden")            // RF-42
	put(s.admin, body(0, 0, -10, 1)).Expect(t, http.StatusBadRequest, "validation_failed")   // RF-37
	put(s.admin, body(10, 0, -101, 1)).Expect(t, http.StatusBadRequest, "validation_failed") // RF-38
	bad := body(10, 0, -10, 1)
	bad["levels"].(map[string]any)["late"] = map[string]any{"color": "pink"}
	put(s.admin, bad).Expect(t, http.StatusBadRequest, "validation_failed") // RF-40
	long := body(10, 0, -10, 1)
	long["levels"].(map[string]any)["ahead"] = map[string]any{"name": "1234567890123456789012345678901", "color": "blue"}
	put(s.admin, long).Expect(t, http.StatusBadRequest, "validation_failed") // RF-39

	var saved struct {
		Version int32 `json:"version"`
		Levels  map[string]struct {
			Name  *string `json:"name"`
			Color string  `json:"color"`
		} `json:"levels"`
	}
	put(s.admin, body(10, 0, -10, 1)).Expect(t, http.StatusOK).JSON(t, &saved)
	if saved.Version != 2 || *saved.Levels["ahead"].Name != "Por delante" || saved.Levels["at_risk"].Name != nil || saved.Levels["ahead"].Color != "violet" {
		t.Fatalf("saved = %+v", saved)
	}
	put(s.admin, body(10, 0, -10, 1)).Expect(t, http.StatusConflict, "version_conflict")
	if acts := s.env.AuditActions("global"); len(acts) != 1 || acts[0] != "schedule_settings.updated" { // RF-50
		t.Fatalf("audit = %v", acts)
	}

	// The next query uses them (RF-41): a gap of −5 is now at risk.
	p := s.project(t, s.lead, map[string]any{"identifier": "SM", "name": "Semáforo"})
	s.item(t, s.member, p.ID, map[string]any{"start_date": "2026-03-01", "due_date": "2026-03-10", "done_ratio": 55})
	if o := s.overview(t, s.viewer, p.ID, "2026-03-06"); o.Figures.Gap != -5 || o.Figures.Level != "at_risk" {
		t.Fatalf("with new thresholds = %+v", o.Figures)
	}
}
