package apitest

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

type refDTO struct {
	Label string `json:"label"`
	State string `json:"state"`
}

type withRefs struct {
	Refs map[string]refDTO `json:"refs"`
}

type mentionResults struct {
	WorkItems  []struct{ ID uuid.UUID } `json:"work_items"`
	Milestones []struct{ ID uuid.UUID } `json:"milestones"`
	Projects   []struct{ ID uuid.UUID } `json:"projects"`
}

func mentionsOf(ms ...[2]string) string {
	doc := `{"type":"doc","content":[{"type":"paragraph","content":[`
	for i, m := range ms {
		if i > 0 {
			doc += ","
		}
		doc += `{"type":"mention","attrs":{"kind":"` + m[0] + `","id":"` + m[1] + `","label":"lo que dijo el editor"}}`
	}
	return doc + `]}]}`
}

// Mentions read with the current names; the reader sees "no access" or
// "deleted" without the title (RF-27 to RF-29).
func TestMentionRefs(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	target := s.item(t, s.member, project, map[string]any{"title": "Autenticación"})
	gone := s.item(t, s.member, project, map[string]any{"title": "Se borrará"})
	act := s.env.CatalogID("work_item_types", "Actividad")
	tpl := s.project(t, s.admin, map[string]any{"identifier": "TPL", "name": "Plantilla secreta", "work_item_type_ids": []uuid.UUID{act}})
	secret := s.item(t, s.admin, tpl.ID, map[string]any{"title": "Interno"})
	s.admin.Post("/api/projects/"+tpl.ID.String()+"/template", nil).Expect(t, http.StatusOK)

	wi := s.item(t, s.member, project, map[string]any{})
	doc := mentionsOf(
		[2]string{"user", s.leadID.String()},
		[2]string{"work_item", target.ID.String()},
		[2]string{"work_item", gone.ID.String()},
		[2]string{"work_item", secret.ID.String()},
		[2]string{"project", project.String()},
		[2]string{"user", uuid.NewString()},
	)
	var saved withRefs
	s.member.Do(http.MethodPut, "/api/work-items/"+wi.ID.String()+"/page", map[string]any{"page": json.RawMessage(doc), "page_version": 1}).
		Expect(t, http.StatusOK).JSON(t, &saved)
	if saved.Refs[s.leadID.String()].Label != "Laura" {
		t.Fatalf("refs on save = %+v", saved.Refs)
	}
	s.member.Do(http.MethodDelete, "/api/work-items/"+gone.ID.String(), nil).Expect(t, http.StatusNoContent)
	s.patch(s.member, target, map[string]any{"title": "Autenticación SSO"}).Expect(t, http.StatusOK)

	var read withRefs
	s.member.Get("/api/work-items/"+wi.ID.String()).Expect(t, http.StatusOK).JSON(t, &read)
	want := map[string]refDTO{
		s.leadID.String():  {Label: "Laura", State: "ok"},
		target.ID.String(): {Label: target.Key + " Autenticación SSO", State: "ok"}, // the current title
		gone.ID.String():   {State: "deleted"},
		secret.ID.String(): {State: "no_access"}, // a member can't use templates
		project.String():   {Label: "KAI Kairos", State: "ok"},
	}
	for id, w := range want {
		if got := read.Refs[id]; got != w {
			t.Errorf("ref %s = %+v, want %+v", id, got, w)
		}
	}
	if len(read.Refs) != 6 {
		t.Errorf("refs = %+v", read.Refs)
	}
	// Whoever may use templates sees it.
	var asAdmin withRefs
	s.admin.Get("/api/work-items/"+wi.ID.String()).JSON(t, &asAdmin)
	if got := asAdmin.Refs[secret.ID.String()]; got.State != "ok" || got.Label == "" {
		t.Errorf("admin ref = %+v", got)
	}
}

// The # search finds work items by key or title, milestones and projects, in
// the reader's scope (RF-26).
func TestMentionSearch(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	a := s.item(t, s.member, project, map[string]any{"title": "Login con SSO"})
	m := s.milestone(t, project, map[string]any{"name": "Login listo", "due_date": "2026-12-01"})
	act := s.env.CatalogID("work_item_types", "Actividad")
	tpl := s.project(t, s.admin, map[string]any{"identifier": "TPL", "name": "Plantilla", "work_item_type_ids": []uuid.UUID{act}})
	s.item(t, s.admin, tpl.ID, map[string]any{"title": "Login de plantilla"})
	s.admin.Post("/api/projects/"+tpl.ID.String()+"/template", nil).Expect(t, http.StatusOK)

	search := func(c *Client, q string) mentionResults {
		t.Helper()
		var r mentionResults
		c.Get("/api/mentions?q="+q).Expect(t, http.StatusOK).JSON(t, &r)
		return r
	}
	r := search(s.member, "login")
	if len(r.WorkItems) != 1 || r.WorkItems[0].ID != a.ID || len(r.Milestones) != 1 || r.Milestones[0].ID != m.ID {
		t.Fatalf("login = %+v", r)
	}
	if r := search(s.member, a.Key); len(r.WorkItems) != 1 || r.WorkItems[0].ID != a.ID {
		t.Fatalf("by key = %+v", r)
	}
	if r := search(s.member, "kai"); len(r.Projects) != 1 || r.Projects[0].ID != project {
		t.Fatalf("project = %+v", r)
	}
	if r := search(s.admin, "login"); len(r.WorkItems) != 2 {
		t.Fatalf("admin sees the template's too: %+v", r)
	}
	if r := search(s.member, "%25"); len(r.WorkItems)+len(r.Milestones)+len(r.Projects) != 0 {
		t.Fatalf("%% is literal: %+v", r)
	}
	if r := search(s.member, ""); r.WorkItems == nil || len(r.WorkItems) != 0 {
		t.Fatalf("empty = %+v", r)
	}
}
