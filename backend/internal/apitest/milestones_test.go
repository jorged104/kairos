package apitest

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

type milestoneDTO struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	CompletedOn *string   `json:"completed_on"`
	Overdue     bool      `json:"overdue"`
	Version     int32     `json:"version"`
	Progress    struct {
		Total          int     `json:"total"`
		Closed         int     `json:"closed"`
		Open           int     `json:"open"`
		Percent        int     `json:"percent"`
		EstimatedHours float64 `json:"estimated_hours"`
		RemainingHours float64 `json:"remaining_hours"`
	} `json:"progress"`
	Can struct {
		Manage bool `json:"manage"`
	} `json:"can"`
}

func (s *workSetup) milestone(t *testing.T, project uuid.UUID, body map[string]any) milestoneDTO {
	t.Helper()
	var m milestoneDTO
	s.lead.Post("/api/projects/"+project.String()+"/milestones", body).Expect(t, http.StatusCreated).JSON(t, &m)
	return m
}

func (s *workSetup) getMilestone(t *testing.T, id uuid.UUID) milestoneDTO {
	t.Helper()
	var m milestoneDTO
	s.viewer.Get("/api/milestones/"+id.String()).Expect(t, http.StatusOK).JSON(t, &m)
	return m
}

func (s *workSetup) link(t *testing.T, c *Client, m uuid.UUID, wi uuid.UUID) *Response {
	t.Helper()
	return c.Post("/api/milestones/"+m.String()+"/work-items", map[string]any{"work_item_id": wi})
}

type effectiveDTO struct {
	Items []struct {
		ID        uuid.UUID `json:"id"`
		Key       string    `json:"key"`
		Inherited bool      `json:"inherited"`
		Name      string    `json:"name"`
	} `json:"items"`
}

func TestMilestoneInheritanceAndProgress(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	m1 := s.milestone(t, project, map[string]any{"name": "M1", "due_date": "2026-06-30"})
	m2 := s.milestone(t, project, map[string]any{"name": "M2"})

	epic := s.item(t, s.member, project, map[string]any{"title": "Épica", "estimated_hours": 10, "remaining_hours": 10})
	subs := make([]workItemDTO, 3)
	for i := range subs {
		subs[i] = s.item(t, s.member, project, map[string]any{"parent_id": epic.ID, "estimated_hours": 5, "remaining_hours": 5})
	}
	s.link(t, s.lead, m1.ID, epic.ID).Expect(t, http.StatusNoContent)
	s.link(t, s.lead, m1.ID, subs[0].ID).Expect(t, http.StatusNoContent) // direct and inherited
	s.link(t, s.lead, m2.ID, subs[1].ID).Expect(t, http.StatusNoContent)

	var eff effectiveDTO
	s.viewer.Get("/api/milestones/"+m1.ID.String()+"/work-items").Expect(t, http.StatusOK).JSON(t, &eff)
	if len(eff.Items) != 4 {
		t.Fatalf("M1 items = %+v, want the epic and its three children once (RF-52)", eff.Items)
	}
	for _, it := range eff.Items {
		wantInherited := it.ID != epic.ID && it.ID != subs[0].ID
		if it.Inherited != wantInherited {
			t.Errorf("%s inherited = %v, want %v", it.Key, it.Inherited, wantInherited)
		}
	}
	// Hours use each item's own value once: 10 + 3×5 (RF-55).
	got := s.getMilestone(t, m1.ID).Progress
	if got.Total != 4 || got.Closed != 0 || got.EstimatedHours != 25 || got.RemainingHours != 25 {
		t.Fatalf("M1 progress = %+v", got)
	}

	// subs[1] counts for M1 (inherited) and M2 (direct) (RF-56).
	var ms effectiveDTO
	s.viewer.Get("/api/work-items/"+subs[1].ID.String()+"/milestones").Expect(t, http.StatusOK).JSON(t, &ms)
	if len(ms.Items) != 2 {
		t.Fatalf("milestones of the subactivity = %+v (RF-53)", ms.Items)
	}
	s.patch(s.member, subs[1], map[string]any{"status_id": s.env.CatalogID("statuses", "Cerrado")}).Expect(t, http.StatusOK)
	if p1, p2 := s.getMilestone(t, m1.ID).Progress, s.getMilestone(t, m2.ID).Progress; p1.Closed != 1 || p1.Percent != 25 || p2.Percent != 100 {
		t.Fatalf("progress after closing = %+v / %+v (RF-54, RF-56)", p1, p2)
	}

	// Moving a child out of the epic drops what it inherited (RF-57).
	s.patch(s.member, s.get(t, subs[2].ID), map[string]any{"parent_id": nil}).Expect(t, http.StatusOK)
	if got := s.getMilestone(t, m1.ID).Progress.Total; got != 3 {
		t.Fatalf("M1 total after the move = %d, want 3", got)
	}

	var list struct {
		Items []milestoneDTO `json:"items"`
	}
	s.viewer.Get("/api/projects/"+project.String()+"/milestones").Expect(t, http.StatusOK).JSON(t, &list)
	if len(list.Items) != 2 || list.Items[0].Progress.Total != 3 {
		t.Fatalf("list = %+v", list.Items)
	}
}

func TestMilestoneRules(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	other := s.project(t, s.lead, map[string]any{"identifier": "OTR", "name": "Otro", "methodology_id": s.env.CatalogID("methodologies", "Scrum")}).ID
	m := s.milestone(t, project, map[string]any{"name": "M1", "due_date": "2020-01-01"})
	if !m.Overdue || !m.Can.Manage {
		t.Fatalf("milestone = %+v (RF-48)", m)
	}
	foreign := s.item(t, s.member, other, map[string]any{})
	s.link(t, s.lead, m.ID, foreign.ID).Expect(t, http.StatusBadRequest, "cross_project") // RF-51

	var phases struct {
		Items []struct {
			ID uuid.UUID `json:"id"`
		} `json:"items"`
	}
	s.lead.Get("/api/projects/"+other.String()+"/phases").JSON(t, &phases)
	s.lead.Post("/api/projects/"+project.String()+"/milestones", map[string]any{"name": "X", "phase_id": phases.Items[0].ID}).
		Expect(t, http.StatusBadRequest, "cross_project") // RF-58

	// Only managers link or edit; members do not (RF-127).
	wi := s.item(t, s.member, project, map[string]any{})
	s.link(t, s.member, m.ID, wi.ID).Expect(t, http.StatusForbidden, "forbidden")
	s.link(t, s.lead, m.ID, wi.ID).Expect(t, http.StatusNoContent)

	// Complete with open items, reopen, cancel (RF-45 to RF-49).
	var done milestoneDTO
	s.lead.Post("/api/milestones/"+m.ID.String()+"/complete", map[string]any{"version": m.Version, "completed_on": "2026-05-01"}).
		Expect(t, http.StatusOK).JSON(t, &done)
	if done.Status != "cumplido" || done.CompletedOn == nil || *done.CompletedOn != "2026-05-01" || done.Overdue || done.Progress.Open != 1 {
		t.Fatalf("completed = %+v", done)
	}
	s.lead.Post("/api/milestones/"+m.ID.String()+"/cancel", map[string]any{"version": done.Version}).Expect(t, http.StatusConflict, "invalid_milestone_status")
	var reopened milestoneDTO
	s.lead.Post("/api/milestones/"+m.ID.String()+"/reopen", map[string]any{"version": done.Version}).Expect(t, http.StatusOK).JSON(t, &reopened)
	if reopened.Status != "abierto" || reopened.CompletedOn != nil {
		t.Fatalf("reopened = %+v", reopened)
	}
	var evs struct {
		Items []struct{ Type string } `json:"items"`
	}
	s.admin.Get("/api/events?subject_id="+m.ID.String()).JSON(t, &evs)
	if len(evs.Items) != 2 || evs.Items[0].Type != "milestone.reopened" || evs.Items[1].Type != "milestone.completed" {
		t.Fatalf("events = %+v (RF-119)", evs.Items)
	}

	// Delete only without links; otherwise cancel (RF-59, RF-60).
	s.lead.Do(http.MethodDelete, "/api/milestones/"+m.ID.String(), nil).Expect(t, http.StatusConflict, "milestone_in_use")
	s.lead.Do(http.MethodDelete, "/api/milestones/"+m.ID.String()+"/work-items/"+wi.ID.String(), nil).Expect(t, http.StatusNoContent)
	s.lead.Do(http.MethodDelete, "/api/milestones/"+m.ID.String(), nil).Expect(t, http.StatusNoContent)

	// Filter of the work item list by effective milestone (RF-129).
	m2 := s.milestone(t, project, map[string]any{"name": "M2"})
	parent := s.item(t, s.member, project, map[string]any{})
	s.item(t, s.member, project, map[string]any{"parent_id": parent.ID})
	s.link(t, s.lead, m2.ID, parent.ID).Expect(t, http.StatusNoContent)
	var page struct {
		Total int `json:"total"`
	}
	s.viewer.Get("/api/projects/"+project.String()+"/work-items?milestone_id="+m2.ID.String()).Expect(t, http.StatusOK).JSON(t, &page)
	if page.Total != 2 {
		t.Fatalf("filtered = %d, want parent and child", page.Total)
	}
}
