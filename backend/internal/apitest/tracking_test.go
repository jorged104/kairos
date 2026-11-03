package apitest

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"
)

type trackingTaskDTO struct {
	Key         string  `json:"key"`
	Title       string  `json:"title"`
	ParentTitle *string `json:"parent_title"`
	LateDays    int     `json:"late_days"`
}

type trackingColumnDTO struct {
	Total int               `json:"total"`
	Items []trackingTaskDTO `json:"items"`
}

type trackingDTO struct {
	Matched  int  `json:"matched"`
	Mine     bool `json:"mine"`
	Projects []struct {
		ID         uuid.UUID `json:"id"`
		Name       string    `json:"name"`
		Milestones []struct {
			Name     string     `json:"name"`
			BarStart *string    `json:"bar_start"`
			BarEnd   *string    `json:"bar_end"`
			Figures  figuresDTO `json:"figures"`
			State    string     `json:"state"`
		} `json:"milestones"`
	} `json:"projects"`
	Summary struct {
		Projects   int        `json:"projects"`
		Milestones int        `json:"milestones"`
		Met        int        `json:"met"`
		Figures    figuresDTO `json:"figures"`
	} `json:"summary"`
	Columns struct {
		Late       trackingColumnDTO `json:"late"`
		InProgress trackingColumnDTO `json:"in_progress"`
		Done       trackingColumnDTO `json:"done"`
		Upcoming   trackingColumnDTO `json:"upcoming"`
	} `json:"columns"`
}

func (s *workSetup) tracking(t *testing.T, c *Client, q url.Values) trackingDTO {
	t.Helper()
	var out trackingDTO
	c.Get("/api/tracking?"+q.Encode()).Expect(t, http.StatusOK).JSON(t, &out)
	return out
}

func TestTracking(t *testing.T) {
	s := newWorkSetup(t)
	today := "2026-03-10"
	alfa := s.project(t, s.lead, map[string]any{"identifier": "ALF", "name": "Alfa"})
	beta := s.project(t, s.lead, map[string]any{"identifier": "BET", "name": "Beta", "owner_id": s.memberID})
	otro := s.project(t, s.admin, map[string]any{"identifier": "OTR", "name": "Otro", "team_id": s.other})

	// Alfa: a parent with a late child and a running one; a milestone on both.
	parent := s.item(t, s.member, alfa.ID, map[string]any{"title": "Padre", "weight": 1})
	late := s.item(t, s.member, alfa.ID, map[string]any{"title": "Vencida", "parent_id": parent.ID, "weight": 1, "due_date": "2026-03-05"})
	s.item(t, s.member, alfa.ID, map[string]any{"title": "En curso", "parent_id": parent.ID, "weight": 1, "start_date": "2026-03-01", "due_date": "2026-03-20"})
	s.item(t, s.member, alfa.ID, map[string]any{"title": "Próxima", "weight": 1, "start_date": "2026-03-15", "due_date": "2026-03-30"})
	m := s.milestone(t, alfa.ID, map[string]any{"name": "Entrega", "due_date": "2026-03-31"})
	s.link(t, s.lead, m.ID, parent.ID).Expect(t, http.StatusNoContent)
	s.milestone(t, beta.ID, map[string]any{"name": "Vacío"})

	// Without filters: the caller's projects (RF-6), or all for someone without any (RF-7).
	if v := s.tracking(t, s.member, url.Values{"today": {today}}); !v.Mine || v.Matched != 2 || v.Projects[0].Name != "Alfa" {
		t.Fatalf("member default = %+v", v)
	}
	nobody := s.env.As(s.env.CreateUser(UserSpec{Email: "nadie@x.com", Roles: []string{"viewer"}}))
	if v := s.tracking(t, nobody, url.Values{"today": {today}}); v.Mine || v.Matched != 3 {
		t.Fatalf("viewer default = matched %d, mine %v", v.Matched, v.Mine)
	}

	// By owner and by project, both at once (RF-3 to RF-5).
	if v := s.tracking(t, s.viewer, url.Values{"today": {today}, "owner_id": {s.memberID.String()}}); v.Matched != 1 || v.Projects[0].Name != "Beta" {
		t.Fatalf("by owner = %+v", v.Projects)
	}
	if v := s.tracking(t, s.viewer, url.Values{"today": {today}, "owner_id": {s.memberID.String()}, "project_id": {alfa.ID.String()}}); v.Matched != 0 {
		t.Fatalf("both = %+v", v.Projects)
	}
	s.viewer.Get("/api/tracking?project_id=x").Expect(t, http.StatusBadRequest, "invalid_filter")

	v := s.tracking(t, s.viewer, url.Values{"today": {today}, "project_id": {alfa.ID.String(), beta.ID.String(), uuid.NewString()}})
	if v.Summary.Projects != 2 || v.Summary.Milestones != 2 || v.Summary.Met != 0 || !v.Summary.Figures.HasPlan { // RF-11 to RF-13
		t.Fatalf("summary = %+v", v.Summary)
	}
	// The milestone's bar comes from its work items; its target is the due date (RF-16, RF-17).
	ms := v.Projects[0].Milestones[0]
	if ms.Name != "Entrega" || *ms.BarStart != "2026-03-01" || *ms.BarEnd != "2026-03-20" || ms.State != "late" {
		t.Fatalf("milestone = %+v", ms)
	}
	if e := v.Projects[1].Milestones[0]; e.State != "no_data" || e.BarStart != nil {
		t.Fatalf("empty milestone = %+v", e)
	}
	// Leaves only, each in its column, with its parent (RF-30 to RF-36).
	c := v.Columns
	if c.Late.Total != 1 || c.Late.Items[0].Key != late.Key || c.Late.Items[0].LateDays != 5 || *c.Late.Items[0].ParentTitle != "Padre" {
		t.Fatalf("late = %+v", c.Late)
	}
	if c.InProgress.Total != 1 || c.Upcoming.Total != 1 || c.Done.Total != 0 {
		t.Fatalf("columns = %+v", c)
	}

	// A program follows its projects (RF-2).
	s.setPortfolio(s.admin, alfa.ID, s.portfolioOf(t, alfa.ID).Version, true, nil).Expect(t, http.StatusOK)
	s.patchProject(s.lead, s.getProject(t, beta.ID), map[string]any{"parent_id": alfa.ID}).Expect(t, http.StatusOK)
	if v := s.tracking(t, s.viewer, url.Values{"today": {today}, "program_id": {alfa.ID.String()}}); v.Matched != 2 {
		t.Fatalf("by program = %+v", v.Projects)
	}
	_ = otro
}

func TestTrackingLimit(t *testing.T) {
	s := newWorkSetup(t)
	for i := range 51 {
		s.project(t, s.admin, map[string]any{"identifier": fmt.Sprintf("L%02d", i), "name": fmt.Sprintf("P%02d", i)})
	}
	v := s.tracking(t, s.env.As(s.env.CreateUser(UserSpec{Email: "nadie@x.com", Roles: []string{"viewer"}})), url.Values{})
	if v.Matched != 51 || len(v.Projects) != 50 || v.Projects[0].Name != "P00" { // RF-9
		t.Fatalf("matched %d, shown %d", v.Matched, len(v.Projects))
	}
}
