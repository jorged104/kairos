package apitest

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

type overviewDTO struct {
	Progress   int        `json:"progress"`
	HasPlan    bool       `json:"has_plan"`
	Expected   int        `json:"expected"`
	Deviation  int        `json:"deviation"`
	Figures    figuresDTO `json:"figures"`
	PlanSeries []struct {
		Day  string  `json:"day"`
		Plan float64 `json:"plan"`
	} `json:"plan_series"`
	Statuses []struct {
		StatusID uuid.UUID `json:"status_id"`
		Count    int       `json:"count"`
	} `json:"statuses"`
	Total         int `json:"total"`
	Closed        int `json:"closed"`
	Overdue       int `json:"overdue"`
	Blocked       int `json:"blocked"`
	NextMilestone *struct {
		Name     string `json:"name"`
		DaysLeft int    `json:"days_left"`
		Percent  int    `json:"percent"`
	} `json:"next_milestone"`
	Milestones []struct {
		Name    string     `json:"name"`
		Figures figuresDTO `json:"figures"`
	} `json:"milestones"`
	Risks []struct {
		Key       string `json:"key"`
		Reason    string `json:"reason"`
		Days      int    `json:"days"`
		Assignees []struct {
			Name string `json:"name"`
		} `json:"assignees"`
	} `json:"risks"`
	Load []struct {
		User struct {
			ID uuid.UUID `json:"id"`
		} `json:"user"`
	} `json:"load"`
	History []struct {
		Day      string `json:"day"`
		Progress int    `json:"progress"`
	} `json:"history"`
}

func (s *workSetup) overview(t *testing.T, c *Client, project uuid.UUID, today string) overviewDTO {
	t.Helper()
	var o overviewDTO
	c.Get("/api/projects/"+project.String()+"/overview?today="+today).Expect(t, http.StatusOK).JSON(t, &o)
	return o
}

func TestOverview(t *testing.T) {
	s := newWorkSetup(t)
	cerrado := s.env.CatalogID("statuses", "Cerrado")
	p := s.project(t, s.lead, map[string]any{"identifier": "KAI", "name": "Kairos", "start_date": "2026-03-01", "end_date": "2026-03-10"})

	// An empty project: 0 %, nothing at risk.
	if o := s.overview(t, s.viewer, p.ID, "2026-03-06"); o.Progress != 0 || o.Total != 0 || len(o.Risks) != 0 || !o.HasPlan || o.Expected != 60 {
		t.Fatalf("empty overview = %+v", o)
	}

	// Weights 3 (closed), 1 (50 %), 1 (0 %) and 0: (300 + 50) / 5 = 70 % (spec 008, Conceptos).
	done := s.item(t, s.member, p.ID, map[string]any{"title": "Hecha", "weight": 3})
	s.patch(s.admin, s.get(t, done.ID), map[string]any{"status_id": cerrado, "weight": 3}).Expect(t, http.StatusOK)
	late := s.item(t, s.member, p.ID, map[string]any{"title": "Vencida", "weight": 1, "done_ratio": 50, "due_date": "2026-03-02",
		"assignee_ids": []uuid.UUID{s.memberID}})
	blocked := s.item(t, s.member, p.ID, map[string]any{"title": "Bloqueada", "weight": 1})
	behind := s.item(t, s.member, p.ID, map[string]any{"title": "Retrasada", "weight": 0, "start_date": "2026-03-01", "due_date": "2026-03-09"})
	s.member.Post("/api/work-items/"+late.ID.String()+"/dependencies", map[string]any{"to_id": blocked.ID, "type": "blocks"}).Expect(t, http.StatusCreated)
	ms := s.milestone(t, p.ID, map[string]any{"name": "Piloto", "due_date": "2026-03-20"})
	s.milestone(t, p.ID, map[string]any{"name": "Diseño", "due_date": "2026-03-05"})
	s.link(t, s.lead, ms.ID, done.ID).Expect(t, http.StatusNoContent)

	o := s.overview(t, s.viewer, p.ID, "2026-03-06")
	// The plan counts the work items with a planned end: «Vencida» (weight 1,
	// due) is all planned and «Retrasada» weighs 0, so 100 % (spec 009, RF-20).
	if o.Progress != 70 || o.Expected != 100 || o.Deviation != -30 || o.Figures.Level != "late" || o.Figures.Gap != -30 {
		t.Fatalf("progress = %+v, want 70 against 100 (spec 009, RF-44)", o)
	}
	if o.Total != 4 || o.Closed != 1 || o.Overdue != 1 || o.Blocked != 1 {
		t.Fatalf("counts = %+v (RF-8, RF-9)", o)
	}
	// The next milestone not met: Diseño was due before but is still open (RF-10, RF-11).
	if o.NextMilestone == nil || o.NextMilestone.Name != "Diseño" || o.NextMilestone.DaysLeft != -1 || len(o.Milestones) != 2 || o.Milestones[0].Name != "Diseño" {
		t.Fatalf("milestones = %+v, next %+v", o.Milestones, o.NextMilestone)
	}
	// Overdue first, then blocked, then behind (RF-15).
	if len(o.Risks) != 3 || o.Risks[0].Key != late.Key || o.Risks[0].Reason != "overdue" || o.Risks[0].Days != 4 ||
		o.Risks[0].Assignees[0].Name != "Mario" || o.Risks[1].Key != blocked.Key || o.Risks[2].Key != behind.Key || o.Risks[2].Reason != "behind" {
		t.Fatalf("risks = %+v", o.Risks)
	}
	if len(o.Load) != 1 || o.Load[0].User.ID != s.memberID {
		t.Fatalf("load = %+v, want only who has open weight (RF-16)", o.Load)
	}

	// Without dates, «Sin plazo» (RF-7).
	noPlan := s.project(t, s.lead, map[string]any{"identifier": "SIN", "name": "Sin plazo"})
	if o := s.overview(t, s.viewer, noPlan.ID, "2026-03-06"); o.HasPlan {
		t.Fatalf("project without dates = %+v", o)
	}

	// Once a day, again on each run (RF-12, RF-13).
	ctx := t.Context()
	day := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	if err := s.env.Store.RecordDailyProgress(ctx, day); err != nil {
		t.Fatal(err)
	}
	s.patch(s.member, s.get(t, blocked.ID), map[string]any{"done_ratio": 100, "weight": 1}).Expect(t, http.StatusOK)
	if err := s.env.Store.RecordDailyProgress(ctx, day); err != nil {
		t.Fatal(err)
	}
	if err := s.env.Store.RecordDailyProgress(ctx, day.AddDate(0, 0, 1)); err != nil {
		t.Fatal(err)
	}
	h := s.overview(t, s.viewer, p.ID, "2026-03-06").History
	if len(h) != 2 || h[0].Day != "2026-03-05" || h[0].Progress != 90 {
		t.Fatalf("history = %+v, want two days, the first updated to 90", h)
	}
	s.outsider.Get("/api/projects/"+p.ID.String()+"/overview?today=ayer").Expect(t, http.StatusBadRequest, "invalid_filter")
}

func TestColumnLimits(t *testing.T) {
	s := newWorkSetup(t)
	p := s.workProject(t)
	enCurso := s.env.CatalogID("statuses", "En progreso")
	set := func(c *Client, v any) *Response {
		return c.Do(http.MethodPut, "/api/projects/"+p.String()+"/column-limits/"+enCurso.String(), map[string]any{"limit": v})
	}
	set(s.member, 4).Expect(t, http.StatusForbidden)
	set(s.lead, 0).Expect(t, http.StatusBadRequest, "validation_failed")
	var proj struct {
		ColumnLimits map[uuid.UUID]int `json:"column_limits"`
	}
	set(s.lead, 4).Expect(t, http.StatusOK).JSON(t, &proj)
	if proj.ColumnLimits[enCurso] != 4 {
		t.Fatalf("limits = %+v", proj.ColumnLimits)
	}
	var cleared struct {
		ColumnLimits map[uuid.UUID]int `json:"column_limits"`
	}
	set(s.lead, nil).Expect(t, http.StatusOK).JSON(t, &cleared)
	if len(cleared.ColumnLimits) != 0 {
		t.Fatalf("limits after clearing = %+v", cleared.ColumnLimits)
	}
}
