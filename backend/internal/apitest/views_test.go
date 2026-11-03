package apitest

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/google/uuid"
)

type viewDTO struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	Shared  bool      `json:"shared"`
	Mine    bool      `json:"mine"`
	CanEdit bool      `json:"can_edit"`
	Count   int       `json:"count"`
	Config  struct {
		Filters map[string][]string `json:"filters"`
		Sort    string              `json:"sort"`
		Group   string              `json:"group"`
	} `json:"config"`
	Unavailable []struct {
		Field string `json:"field"`
		Value string `json:"value"`
	} `json:"unavailable"`
}

type viewsDTO struct {
	Items    []viewDTO `json:"items"`
	AllCount int       `json:"all_count"`
}

func (s *workSetup) views(t *testing.T, c *Client, project uuid.UUID) viewsDTO {
	t.Helper()
	var out viewsDTO
	c.Get("/api/projects/"+project.String()+"/views").Expect(t, http.StatusOK).JSON(t, &out)
	return out
}

func (s *workSetup) saveView(c *Client, project uuid.UUID, name string, shared bool, filters map[string][]string) *Response {
	return c.Post("/api/projects/"+project.String()+"/views", map[string]any{
		"name": name, "shared": shared, "config": map[string]any{"filters": filters, "sort": "-due", "group": "status"}})
}

func TestSavedViews(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	front := s.label(t, "Frontend")
	s.item(t, s.member, project, map[string]any{"assignee_ids": []uuid.UUID{s.memberID}, "label_ids": []uuid.UUID{front}})
	s.item(t, s.member, project, map[string]any{"assignee_ids": []uuid.UUID{s.leadID}})
	s.item(t, s.member, project, map[string]any{})

	// Anyone saves personal views; "me" is whoever looks (RF-19, RF-24).
	var mine viewDTO
	s.saveView(s.member, project, "Mis tareas", false, map[string][]string{"assignee_id": {"me"}}).Expect(t, http.StatusCreated).JSON(t, &mine)
	if mine.Count != 1 || mine.Shared || !mine.Mine || !mine.CanEdit || mine.Config.Group != "status" {
		t.Fatalf("personal view = %+v", mine)
	}
	s.saveView(s.viewer, project, "Para leer", false, map[string][]string{"closed": {"false"}}).Expect(t, http.StatusCreated)

	// Only who manages the team shares them (RF-25).
	s.saveView(s.member, project, "Frontend", true, map[string][]string{"label_id": {front.String()}}).Expect(t, http.StatusForbidden)
	var shared viewDTO
	s.saveView(s.lead, project, "Frontend", true, map[string][]string{"label_id": {front.String()}}).Expect(t, http.StatusCreated).JSON(t, &shared)
	s.saveView(s.lead, project, "Las mías", true, map[string][]string{"assignee_id": {"me"}}).Expect(t, http.StatusCreated)

	// Each one sees «Todas», the shared ones and their own (RF-23, RF-26).
	got := s.views(t, s.member, project)
	if got.AllCount != 3 || len(got.Items) != 3 {
		t.Fatalf("member's views = %+v", got)
	}
	for _, v := range got.Items {
		switch v.Name {
		case "Frontend":
			if !v.Shared || v.CanEdit || v.Count != 1 {
				t.Fatalf("shared seen by the member = %+v", v)
			}
		case "Las mías":
			if v.Count != 1 { // the member's own items, not the lead's
				t.Fatalf("shared «me» view for the member = %+v", v)
			}
		}
	}
	if n := len(s.views(t, s.lead, project).Items); n != 2 {
		t.Fatalf("lead sees %d views, want the 2 shared", n)
	}

	// Only the author changes a personal view; a shared one, its managers (RF-28).
	patch := func(c *Client, v viewDTO, name string) *Response {
		return c.Do(http.MethodPatch, "/api/views/"+v.ID.String(), map[string]any{"name": name, "shared": v.Shared,
			"config": map[string]any{"filters": v.Config.Filters}})
	}
	patch(s.lead, mine, "Robada").Expect(t, http.StatusNotFound)
	patch(s.member, shared, "Otra").Expect(t, http.StatusForbidden)
	patch(s.member, mine, "Abiertas").Expect(t, http.StatusOK)
	patch(s.lead, shared, "Frontend 2").Expect(t, http.StatusOK)

	// Names are unique among what someone sees, «Todas» included (RF-29).
	s.saveView(s.member, project, " frontend 2 ", false, nil).Expect(t, http.StatusConflict, "view_name_conflict")
	s.saveView(s.member, project, "TODAS", false, nil).Expect(t, http.StatusConflict, "view_name_conflict")
	s.saveView(s.lead, project, "abiertas", true, nil).Expect(t, http.StatusConflict, "view_name_conflict")
	s.saveView(s.lead, project, "Abiertas", false, nil).Expect(t, http.StatusCreated) // the lead's own

	// Bad configs are rejected.
	s.saveView(s.member, project, "Mala", false, map[string][]string{"color": {"rojo"}}).Expect(t, http.StatusBadRequest, "validation_failed")
	s.saveView(s.member, project, "Mala", false, map[string][]string{"status_id": {"x"}}).Expect(t, http.StatusBadRequest, "validation_failed")
	s.member.Post("/api/projects/"+project.String()+"/views", map[string]any{"name": "Mala", "config": map[string]any{"group": "color"}}).
		Expect(t, http.StatusBadRequest, "validation_failed")

	// A retired value is left out and marked (RF-31).
	s.admin.Post("/api/labels/"+front.String()+"/retire", nil).Expect(t, http.StatusOK)
	for _, v := range s.views(t, s.member, project).Items {
		if v.Name == "Frontend 2" && (len(v.Unavailable) != 1 || v.Count != 3) {
			t.Fatalf("view with a retired label = %+v, want it unavailable and the rest applied", v)
		}
	}

	// At most 50 personal views per person (RF-30).
	for i := 2; i <= 50; i++ {
		s.saveView(s.viewer, project, "V"+strconv.Itoa(i), false, nil).Expect(t, http.StatusCreated)
	}
	s.saveView(s.viewer, project, "Una más", false, nil).Expect(t, http.StatusConflict, "too_many_views")

	// Deleting: the author's, or managers for shared ones.
	s.lead.Do(http.MethodDelete, "/api/views/"+mine.ID.String(), nil).Expect(t, http.StatusNotFound)
	s.member.Do(http.MethodDelete, "/api/views/"+mine.ID.String(), nil).Expect(t, http.StatusNoContent)
	s.lead.Do(http.MethodDelete, "/api/views/"+shared.ID.String(), nil).Expect(t, http.StatusNoContent)
}
