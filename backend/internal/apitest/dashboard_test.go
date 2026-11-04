package apitest

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

type dashboardDTO struct {
	Today   string `json:"today"`
	Profile struct {
		Open        int `json:"open"`
		Late        int `json:"late"`
		ClosedMonth int `json:"closed_month"`
	} `json:"profile"`
	Tasks []struct {
		ID       uuid.UUID `json:"id"`
		Title    string    `json:"title"`
		Closed   bool      `json:"closed"`
		LateDays int       `json:"late_days"`
		ClosedOn *string   `json:"closed_on"`
		CanMove  bool      `json:"can_move"`
		Project  struct {
			Name string `json:"name"`
		} `json:"project"`
	} `json:"tasks"`
	Projects []struct {
		Name    string     `json:"name"`
		Role    string     `json:"role"`
		Open    int        `json:"open"`
		Late    int        `json:"late"`
		Figures figuresDTO `json:"figures"`
	} `json:"projects"`
	Weeks []struct {
		Start   string `json:"start"`
		Closed  int    `json:"closed"`
		Weight  int    `json:"weight"`
		WithDue int    `json:"with_due"`
		OnTime  int    `json:"on_time"`
	} `json:"weeks"`
	Load []struct {
		Project *struct {
			Name string `json:"name"`
		} `json:"project"`
		Open int `json:"open"`
		Late int `json:"late"`
	} `json:"load"`
}

func (s *workSetup) dashboard(t *testing.T, c *Client, today string) dashboardDTO {
	t.Helper()
	var out dashboardDTO
	c.Get("/api/me/dashboard?today="+today).Expect(t, http.StatusOK).JSON(t, &out)
	return out
}

func TestDashboard(t *testing.T) {
	s := newWorkSetup(t)
	me := []uuid.UUID{s.outsiderID}
	closed := s.env.CatalogID("statuses", "Cerrado")
	closeAt := func(id uuid.UUID, at string) {
		s.exec(t, "UPDATE work_items SET status_id = $1, closed_at = $2::timestamptz WHERE id = $3", closed, at, id)
	}

	alfa := s.project(t, s.lead, map[string]any{"identifier": "ALF", "name": "Alfa"})
	s.item(t, s.lead, alfa.ID, map[string]any{"title": "Atrasada", "assignee_ids": me, "due_date": "2026-03-01"})
	s.item(t, s.lead, alfa.ID, map[string]any{"title": "Sin fecha", "assignee_ids": me})
	late := s.item(t, s.lead, alfa.ID, map[string]any{"title": "Cerrada tarde", "assignee_ids": me, "due_date": "2026-03-04", "weight": 3})
	closeAt(late.ID, "2026-03-05T10:00:00Z")
	onTime := s.item(t, s.lead, alfa.ID, map[string]any{"title": "A tiempo", "assignee_ids": me, "due_date": "2026-02-10"})
	closeAt(onTime.ID, "2026-02-01T10:00:00Z")
	old := s.item(t, s.lead, alfa.ID, map[string]any{"title": "Antigua", "assignee_ids": me})
	closeAt(old.ID, "2025-11-01T10:00:00Z")
	s.item(t, s.lead, alfa.ID, map[string]any{"title": "De otro", "assignee_ids": []uuid.UUID{s.memberID}})

	beta := s.project(t, s.admin, map[string]any{"identifier": "BET", "name": "Beta", "team_id": s.other, "owner_id": s.outsiderID})
	s.item(t, s.outsider, beta.ID, map[string]any{"title": "Mía", "assignee_ids": me, "due_date": "2026-03-20"})
	gamma := s.project(t, s.lead, map[string]any{"identifier": "GAM", "name": "Gamma"})
	s.item(t, s.lead, gamma.ID, map[string]any{"title": "De un cerrado", "assignee_ids": me})
	s.lead.Post("/api/projects/"+gamma.ID.String()+"/close", map[string]any{"version": gamma.Version}).Expect(t, http.StatusOK)
	delta := s.project(t, s.lead, map[string]any{"identifier": "DEL", "name": "Delta"})
	s.addMember(s.lead, delta.ID, s.outsiderID, "observer").Expect(t, http.StatusOK)

	d := s.dashboard(t, s.outsider, "2026-03-11")
	titles := map[string]int{}
	for _, task := range d.Tasks {
		titles[task.Title] = task.LateDays
	}
	// Open ones and the ones closed in the last 14 days, of active projects (RF-10, RF-11).
	if len(d.Tasks) != 4 || titles["Atrasada"] != 10 || titles["Sin fecha"] != 0 {
		t.Fatalf("tasks = %+v", d.Tasks)
	}
	for _, name := range []string{"Cerrada tarde", "Mía"} {
		if _, ok := titles[name]; !ok {
			t.Fatalf("missing %q in %+v", name, d.Tasks)
		}
	}
	if d.Profile.Open != 3 || d.Profile.Late != 1 || d.Profile.ClosedMonth != 1 { // RF-6
		t.Fatalf("profile = %+v", d.Profile)
	}
	// 12 weeks from Monday to Sunday, this one last (RF-28 to RF-31).
	if len(d.Weeks) != 12 || d.Weeks[11].Start != "2026-03-09" || d.Weeks[0].Start != "2025-12-22" {
		t.Fatalf("weeks = %+v", d.Weeks)
	}
	if w := d.Weeks[10]; w.Closed != 1 || w.Weight != 3 || w.WithDue != 1 || w.OnTime != 0 {
		t.Fatalf("week of the late close = %+v", w)
	}
	if w := d.Weeks[5]; w.Closed != 1 || w.OnTime != 1 {
		t.Fatalf("week of the on-time close = %+v", w)
	}
	// The projects: the most late first, with the role (RF-23, RF-24).
	got := []string{}
	for _, p := range d.Projects {
		got = append(got, p.Name+":"+p.Role)
	}
	if !equal(got, []string{"Alfa:tasks", "Beta:owner", "Delta:observer"}) || d.Projects[0].Open != 2 || d.Projects[0].Late != 1 {
		t.Fatalf("projects = %+v", d.Projects)
	}
	if len(d.Load) != 2 || d.Load[0].Project.Name != "Alfa" || d.Load[0].Open != 2 || d.Load[0].Late != 1 { // RF-33
		t.Fatalf("load = %+v", d.Load)
	}

	// Each person sees only their own (RF-3).
	if other := s.dashboard(t, s.member, "2026-03-11"); len(other.Tasks) != 1 || other.Tasks[0].Title != "De otro" {
		t.Fatalf("member tasks = %+v", other.Tasks)
	}
	s.outsider.Get("/api/me/dashboard?today=hoy").Expect(t, http.StatusBadRequest)
}

// More than 8 projects with open work items: the rest go to «Otros» (RF-34).
func TestDashboardLoadOthers(t *testing.T) {
	s := newWorkSetup(t)
	for i := range 10 {
		p := s.project(t, s.lead, map[string]any{"identifier": "P" + string(rune('A'+i)), "name": "Proyecto " + string(rune('A'+i))})
		for range 10 - i {
			s.item(t, s.lead, p.ID, map[string]any{"title": "T", "assignee_ids": []uuid.UUID{s.memberID}})
		}
	}
	d := s.dashboard(t, s.member, "2026-03-11")
	if len(d.Load) != 9 || d.Load[0].Project.Name != "Proyecto A" || d.Load[8].Project != nil || d.Load[8].Open != 3 {
		t.Fatalf("load = %+v", d.Load)
	}
}
