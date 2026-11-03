package apitest

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

type metricsDTO struct {
	Schedule *struct {
		HasPlan   bool   `json:"has_plan"`
		Expected  int    `json:"expected"`
		Actual    int    `json:"actual"`
		Deviation int    `json:"deviation"`
		State     string `json:"state"`
	} `json:"schedule"`
	Contribution struct {
		Tracking bool      `json:"tracking"`
		Parent   *shareDTO `json:"parent"`
		Project  shareDTO  `json:"project"`
	} `json:"contribution"`
	Times struct {
		Closed         bool     `json:"closed"`
		LeadDays       *float64 `json:"lead_days"`
		CycleDays      *float64 `json:"cycle_days"`
		AgeDays        *float64 `json:"age_days"`
		InProgressDays *float64 `json:"in_progress_days"`
		InStatusDays   *float64 `json:"in_status_days"`
		Missing        bool     `json:"missing"`
	} `json:"times"`
	Due struct {
		State string `json:"state"`
		Days  int    `json:"days"`
	} `json:"due"`
	Reopens *int `json:"reopens"`
	Blocked struct {
		Now      bool `json:"now"`
		Blockers []struct {
			Hidden bool   `json:"hidden"`
			Key    string `json:"key"`
		} `json:"blockers"`
		TotalDays *float64 `json:"total_days"`
	} `json:"blocked"`
}

type shareDTO struct {
	NoWeight bool    `json:"no_weight"`
	Percent  float64 `json:"percent"`
	Points   float64 `json:"points"`
}

func (s *workSetup) metrics(t *testing.T, c *Client, id uuid.UUID, today string) metricsDTO {
	t.Helper()
	var m metricsDTO
	c.Get("/api/work-items/"+id.String()+"/metrics?today="+today).Expect(t, http.StatusOK).JSON(t, &m)
	return m
}

// exec runs SQL to move times into the past. The audit is insert-only, so
// its triggers are off meanwhile (each test has its own database).
func (s *workSetup) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	for _, step := range []struct {
		sql  string
		args []any
	}{{"ALTER TABLE audit_log DISABLE TRIGGER USER", nil}, {sql, args}, {"ALTER TABLE audit_log ENABLE TRIGGER USER", nil}} {
		if _, err := s.env.Store.Pool.Exec(t.Context(), step.sql, step.args...); err != nil {
			t.Fatal(err)
		}
	}
}

func near(v *float64, want float64) bool { return v != nil && *v >= want-0.15 && *v <= want+0.15 }

func TestMetricsSchedule(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	// From the 1st to the 10th: on the 6th, 60 % of the time has gone (RF-10).
	wi := s.item(t, s.member, project, map[string]any{"start_date": "2026-03-01", "due_date": "2026-03-10", "done_ratio": 20})
	cases := []struct {
		today, state string
		ratio, want  int
	}{
		// The four levels of the traffic light (spec 009, RF-28 to RF-31, RF-43).
		{"2026-03-06", "late", 20, 60},
		{"2026-03-06", "at_risk", 50, 60},
		{"2026-03-06", "on_track", 55, 60},
		{"2026-03-06", "ahead", 75, 60},
		{"2026-02-20", "ahead", 20, 0},  // before the start (spec 009, RF-14)
		{"2026-04-01", "late", 75, 100}, // after the end (spec 009, RF-15)
	}
	for _, c := range cases {
		s.patch(s.member, s.get(t, wi.ID), map[string]any{"done_ratio": c.ratio, "start_date": "2026-03-01", "due_date": "2026-03-10"}).Expect(t, http.StatusOK)
		m := s.metrics(t, s.viewer, wi.ID, c.today)
		if m.Schedule == nil || !m.Schedule.HasPlan || m.Schedule.Expected != c.want || m.Schedule.State != c.state ||
			m.Schedule.Deviation != c.ratio-c.want {
			t.Fatalf("%s at %d %% = %+v, want %d %% expected and %s", c.today, c.ratio, m.Schedule, c.want, c.state)
		}
	}
	// Without dates, «Sin plazo» (RF-14); closed, no schedule (RF-15).
	none := s.item(t, s.member, project, map[string]any{"start_date": "2026-03-01"})
	if m := s.metrics(t, s.member, none.ID, "2026-03-06"); m.Schedule == nil || m.Schedule.HasPlan {
		t.Fatalf("without due date = %+v, want no plan", m.Schedule)
	}
	s.patch(s.admin, s.get(t, wi.ID), map[string]any{"status_id": s.env.CatalogID("statuses", "Cerrado")}).Expect(t, http.StatusOK)
	if m := s.metrics(t, s.member, wi.ID, "2026-03-06"); m.Schedule != nil {
		t.Fatalf("closed = %+v, want no schedule", m.Schedule)
	}
	s.member.Get("/api/work-items/"+wi.ID.String()+"/metrics?today=ayer").Expect(t, http.StatusBadRequest, "invalid_filter")
}

func TestMetricsContribution(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	// Parent 1 + child 3, plus another root of 1: the project weighs 5.
	parent := s.item(t, s.member, project, map[string]any{"weight": 1})
	child := s.item(t, s.member, project, map[string]any{"weight": 3, "parent_id": parent.ID, "done_ratio": 50})
	s.item(t, s.member, project, map[string]any{"weight": 1})

	m := s.metrics(t, s.member, child.ID, "2026-03-06")
	if m.Contribution.Parent == nil || m.Contribution.Parent.Percent != 75 || m.Contribution.Parent.Points != 37.5 {
		t.Fatalf("to the parent = %+v, want 75 %% and 37.5 points (RF-16, RF-18)", m.Contribution.Parent)
	}
	if m.Contribution.Project.Percent != 60 || m.Contribution.Project.Points != 30 || m.Contribution.Tracking {
		t.Fatalf("to the project = %+v, want 60 %% and 30 points (RF-17)", m.Contribution.Project)
	}
	// A root has no parent; the parent counts its whole subtree.
	if m := s.metrics(t, s.member, parent.ID, "2026-03-06"); m.Contribution.Parent != nil || m.Contribution.Project.Percent != 80 {
		t.Fatalf("root = %+v, want no parent and 80 %%", m.Contribution)
	}
	// Weight 0: tracking (RF-19).
	s.patch(s.member, s.get(t, child.ID), map[string]any{"weight": 0}).Expect(t, http.StatusOK)
	if m := s.metrics(t, s.member, child.ID, "2026-03-06"); !m.Contribution.Tracking || m.Contribution.Project.Percent != 0 {
		t.Fatalf("weight 0 = %+v, want tracking", m.Contribution)
	}
	// A project without weight (RF-20).
	empty := s.project(t, s.lead, map[string]any{"identifier": "VAC", "name": "Vacío"}).ID
	zero := s.item(t, s.member, empty, map[string]any{"weight": 0})
	if m := s.metrics(t, s.member, zero.ID, "2026-03-06"); !m.Contribution.Project.NoWeight {
		t.Fatalf("project without weight = %+v, want no weight", m.Contribution.Project)
	}
}

func TestMetricsTimesAndDue(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	cerrado, enCurso, nuevo := s.env.CatalogID("statuses", "Cerrado"), s.env.CatalogID("statuses", "En progreso"), s.env.CatalogID("statuses", "Nuevo")

	// Open: days to the due date and overdue (RF-26, RF-27); none without it (RF-29).
	wi := s.item(t, s.member, project, map[string]any{"due_date": "2026-03-10"})
	if m := s.metrics(t, s.member, wi.ID, "2026-03-06"); m.Due.State != "pending" || m.Due.Days != 4 {
		t.Fatalf("due = %+v, want 4 days left", m.Due)
	}
	if m := s.metrics(t, s.member, wi.ID, "2026-03-13"); m.Due.State != "overdue" || m.Due.Days != 3 {
		t.Fatalf("due = %+v, want 3 days overdue", m.Due)
	}
	if m := s.metrics(t, s.member, s.item(t, s.member, project, map[string]any{}).ID, "2026-03-06"); m.Due.State != "none" {
		t.Fatalf("due = %+v, want none", m.Due)
	}

	// Started, closed, reopened and closed again (RF-21 to RF-25, RF-30).
	s.patch(s.admin, s.get(t, wi.ID), map[string]any{"status_id": enCurso, "due_date": "2026-03-10"}).Expect(t, http.StatusOK)
	m := s.metrics(t, s.member, wi.ID, "2026-03-06")
	if m.Times.Closed || m.Times.AgeDays == nil || m.Times.InProgressDays == nil || m.Times.InStatusDays == nil || *m.Reopens != 0 {
		t.Fatalf("open times = %+v, reopens %v", m.Times, m.Reopens)
	}
	s.patch(s.admin, s.get(t, wi.ID), map[string]any{"status_id": cerrado, "due_date": "2026-03-10"}).Expect(t, http.StatusOK)
	s.patch(s.admin, s.get(t, wi.ID), map[string]any{"status_id": nuevo, "due_date": "2026-03-10"}).Expect(t, http.StatusOK)
	s.patch(s.admin, s.get(t, wi.ID), map[string]any{"status_id": cerrado, "due_date": "2026-03-10"}).Expect(t, http.StatusOK)
	// Created 10 days ago and started 8 days ago.
	s.exec(t, "UPDATE work_items SET created_at = now() - interval '10 days' WHERE id = $1", wi.ID)
	s.exec(t, `UPDATE audit_log SET occurred_at = now() - interval '8 days' WHERE id = (
		SELECT min(id) FROM audit_log WHERE target_id = $1 AND action = 'work_item.updated')`, wi.ID.String())
	m = s.metrics(t, s.member, wi.ID, "2026-03-06")
	if !m.Times.Closed || !near(m.Times.LeadDays, 10) || !near(m.Times.CycleDays, 8) || m.Reopens == nil || *m.Reopens != 1 {
		t.Fatalf("closed times = %+v, reopens %v, want lead 10, cycle 8 and 1 reopen", m.Times, *m.Reopens)
	}
	// Closed after its due date of 2026-03-10 (RF-28).
	if m.Due.State != "late" || m.Due.Days < 1 {
		t.Fatalf("due = %+v, want late", m.Due)
	}
	onTime := s.item(t, s.member, project, map[string]any{"due_date": time.Now().AddDate(0, 0, 5).Format(time.DateOnly)})
	s.patch(s.admin, s.get(t, onTime.ID), map[string]any{"status_id": cerrado, "due_date": time.Now().AddDate(0, 0, 5).Format(time.DateOnly)}).Expect(t, http.StatusOK)
	m = s.metrics(t, s.member, onTime.ID, "2026-03-06")
	if m.Due.State != "on_time" {
		t.Fatalf("due = %+v, want on time", m.Due)
	}
	// Closed straight from the initial status: no cycle time (RF-24).
	if m.Times.LeadDays == nil || m.Times.CycleDays != nil {
		t.Fatalf("closed from new = %+v, want lead and no cycle", m.Times)
	}

	// A change without its status in the audit: «Sin datos» (RF-35).
	s.exec(t, `INSERT INTO audit_log (actor_kind, action, target_type, target_id, before, after)
		VALUES ('system', 'work_item.updated', 'work_item', $1, '{}', '{}')`, onTime.ID.String())
	m = s.metrics(t, s.member, onTime.ID, "2026-03-06")
	if !m.Times.Missing || m.Reopens != nil || m.Times.InStatusDays != nil || m.Times.LeadDays == nil {
		t.Fatalf("with missing history = %+v, reopens %v", m.Times, m.Reopens)
	}
}

func TestMetricsBlocked(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	cerrado := s.env.CatalogID("statuses", "Cerrado")
	wi := s.item(t, s.member, project, map[string]any{})
	blocker := s.item(t, s.member, project, map[string]any{"title": "Antes"})
	s.member.Post("/api/work-items/"+blocker.ID.String()+"/dependencies", map[string]any{"to_id": wi.ID, "type": "blocks"}).Expect(t, http.StatusCreated)

	// Blocked by an open work item (RF-31).
	m := s.metrics(t, s.member, wi.ID, "2026-03-06")
	if !m.Blocked.Now || len(m.Blocked.Blockers) != 1 || m.Blocked.Blockers[0].Key != "KAI-2" || m.Blocked.TotalDays == nil {
		t.Fatalf("blocked = %+v", m.Blocked)
	}

	// Blocked from 5 days ago until the blocker closed 2 days ago: 3 days (RF-33).
	s.patch(s.admin, s.get(t, blocker.ID), map[string]any{"status_id": cerrado}).Expect(t, http.StatusOK)
	s.exec(t, "UPDATE work_items SET created_at = now() - interval '7 days' WHERE id = ANY ($1)", []uuid.UUID{wi.ID, blocker.ID})
	s.exec(t, "UPDATE audit_log SET occurred_at = now() - interval '5 days' WHERE action = 'work_item.dependency_added' AND target_id = $1", blocker.ID.String())
	s.exec(t, "UPDATE audit_log SET occurred_at = now() - interval '2 days' WHERE action = 'work_item.updated' AND target_id = $1", blocker.ID.String())
	m = s.metrics(t, s.member, wi.ID, "2026-03-06")
	if m.Blocked.Now || len(m.Blocked.Blockers) != 0 || !near(m.Blocked.TotalDays, 3) {
		t.Fatalf("blocked = %+v, want 3 days in total", m.Blocked)
	}

	// Not blocked ever: 0.
	if m := s.metrics(t, s.member, blocker.ID, "2026-03-06"); m.Blocked.TotalDays == nil || *m.Blocked.TotalDays != 0 {
		t.Fatalf("never blocked = %+v", m.Blocked)
	}

	// A blocker the caller can't see counts without its key (RF-32).
	tpl := s.project(t, s.admin, map[string]any{"identifier": "TPL", "name": "Plantilla"})
	hidden := s.item(t, s.admin, tpl.ID, map[string]any{})
	s.admin.Post("/api/work-items/"+hidden.ID.String()+"/dependencies", map[string]any{"to_id": wi.ID, "type": "blocks"}).Expect(t, http.StatusCreated)
	s.admin.Post("/api/projects/"+tpl.ID.String()+"/template", nil).Expect(t, http.StatusOK)
	m = s.metrics(t, s.member, wi.ID, "2026-03-06")
	if !m.Blocked.Now || len(m.Blocked.Blockers) != 1 || !m.Blocked.Blockers[0].Hidden || m.Blocked.Blockers[0].Key != "" {
		t.Fatalf("hidden blocker = %+v", m.Blocked)
	}
}
