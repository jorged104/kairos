package apitest

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestTemplates(t *testing.T) {
	s := newWorkSetup(t)
	prince := s.env.CatalogID("methodologies", "PRINCE2")
	act := s.env.CatalogID("work_item_types", "Actividad")
	tpl := s.project(t, s.admin, map[string]any{"identifier": "TPL", "name": "Plantilla PRINCE2", "methodology_id": prince,
		"start_date": "2026-01-01", "work_item_type_ids": []uuid.UUID{act}})

	// Content of the template: an epic with two children, a dependency, a milestone.
	m := s.milestone(t, tpl.ID, map[string]any{"name": "Entrega", "due_date": "2026-01-31"})
	epic := s.item(t, s.admin, tpl.ID, map[string]any{"title": "Épica", "estimated_hours": 4, "assignee_id": s.memberID})
	a := s.item(t, s.admin, tpl.ID, map[string]any{"title": "A", "parent_id": epic.ID, "start_date": "2026-01-05", "estimated_hours": 2, "weight": 7})
	b := s.item(t, s.admin, tpl.ID, map[string]any{"title": "B", "parent_id": epic.ID})
	s.admin.Post("/api/work-items/"+a.ID.String()+"/dependencies", map[string]any{"to_id": b.ID, "type": "precedes"}).Expect(t, http.StatusCreated)
	s.link(t, s.admin, m.ID, epic.ID).Expect(t, http.StatusNoContent)

	// Only admins mark templates; then they leave the project list (RF-34, RF-35).
	s.lead.Post("/api/projects/"+tpl.ID.String()+"/template", nil).Expect(t, http.StatusForbidden, "forbidden")
	s.admin.Post("/api/projects/"+tpl.ID.String()+"/template", nil).Expect(t, http.StatusOK)
	var list struct {
		Total int `json:"total"`
	}
	s.lead.Get("/api/projects").JSON(t, &list)
	if list.Total != 0 {
		t.Fatalf("templates listed among projects: %d", list.Total)
	}
	var tpls struct {
		Items []struct{ Identifier string } `json:"items"`
	}
	s.lead.Get("/api/templates").Expect(t, http.StatusOK).JSON(t, &tpls)
	if len(tpls.Items) != 1 {
		t.Fatalf("templates = %+v", tpls.Items)
	}
	s.member.Get("/api/templates").Expect(t, http.StatusForbidden)
	s.member.Get("/api/projects/"+tpl.ID.String()).Expect(t, http.StatusNotFound)

	// Changes inside a template emit no events (RF-35).
	s.patch(s.admin, s.get(t, b.ID), map[string]any{"status_id": s.env.CatalogID("statuses", "Cerrado")}).Expect(t, http.StatusOK)
	var evs struct {
		Total int `json:"total"`
	}
	s.admin.Get("/api/events").JSON(t, &evs)
	if evs.Total != 0 {
		t.Fatalf("template emitted %d events", evs.Total)
	}

	// Copy with content, starting one month later (RF-36 to RF-38).
	var bir projectDTO
	s.lead.Post("/api/projects/from-template", map[string]any{"template_id": tpl.ID, "identifier": "BIR", "name": "BI Reportes",
		"team_id": s.bi, "start_date": "2026-02-01", "include_content": true}).Expect(t, http.StatusCreated).JSON(t, &bir)
	if bir.Methodology == nil || bir.Methodology.Name != "PRINCE2" || len(bir.TypeIDs) != 1 {
		t.Fatalf("copy = %+v", bir)
	}
	var tree struct {
		Items []struct {
			Key      string     `json:"key"`
			Title    string     `json:"title"`
			ParentID *uuid.UUID `json:"parent_id"`
			StatusID uuid.UUID  `json:"status_id"`
			Assignee *struct{}  `json:"assignee"`
		} `json:"items"`
	}
	s.lead.Get("/api/projects/"+bir.ID.String()+"/tree").Expect(t, http.StatusOK).JSON(t, &tree)
	if len(tree.Items) != 3 || tree.Items[0].Key != "BIR-1" || tree.Items[1].ParentID == nil {
		t.Fatalf("copied tree = %+v", tree.Items)
	}
	for _, it := range tree.Items {
		if it.StatusID != s.env.CatalogID("statuses", "Nuevo") || it.Assignee != nil {
			t.Fatalf("copied item not reset: %+v", it)
		}
	}
	var copiedA workItemDTO
	s.lead.Get("/api/work-items/by-key/BIR-2").JSON(t, &copiedA)
	var raw struct {
		StartDate    string `json:"start_date"`
		Weight       int    `json:"weight"`
		Dependencies []struct {
			Type string `json:"type"`
		} `json:"dependencies"`
	}
	s.lead.Get("/api/work-items/"+copiedA.ID.String()).JSON(t, &raw)
	if raw.StartDate != "2026-02-05" || len(raw.Dependencies) != 1 || raw.Weight != 7 {
		t.Fatalf("copied A = %+v, want dates shifted 31 days, its dependency and its weight (RF-157)", raw)
	}
	var ms struct {
		Items []milestoneDTO `json:"items"`
	}
	s.lead.Get("/api/projects/"+bir.ID.String()+"/milestones").JSON(t, &ms)
	if len(ms.Items) != 1 || ms.Items[0].Progress.Total != 3 {
		t.Fatalf("copied milestones = %+v", ms.Items)
	}
	// New work items continue the numbering.
	if got := s.item(t, s.lead, bir.ID, map[string]any{"type_id": act}); got.Key != "BIR-4" {
		t.Fatalf("next key = %s", got.Key)
	}

	// Without content, and later template changes do not reach copies (RF-39).
	var bis projectDTO
	s.lead.Post("/api/projects/from-template", map[string]any{"template_id": tpl.ID, "identifier": "BIS", "name": "Otro", "team_id": s.bi}).
		Expect(t, http.StatusCreated).JSON(t, &bis)
	s.lead.Get("/api/projects/"+bis.ID.String()+"/tree").JSON(t, &tree)
	if len(tree.Items) != 0 {
		t.Fatalf("copy without content has %d items", len(tree.Items))
	}
	s.admin.Do(http.MethodPut, "/api/projects/"+tpl.ID.String()+"/types", map[string]any{"ids": []uuid.UUID{act, s.env.CatalogID("work_item_types", "Bug")}}).
		Expect(t, http.StatusOK)
	var again projectDTO
	s.lead.Get("/api/projects/"+bir.ID.String()).JSON(t, &again)
	if len(again.TypeIDs) != 1 {
		t.Fatalf("template change reached a copy: %v", again.TypeIDs)
	}

	// Members cannot use templates; a normal project is not a template; templates take no objectives.
	s.member.Post("/api/projects/from-template", map[string]any{"template_id": tpl.ID, "identifier": "MEM", "name": "X", "team_id": s.rep}).
		Expect(t, http.StatusForbidden)
	s.lead.Post("/api/projects/from-template", map[string]any{"template_id": bir.ID, "identifier": "NOT", "name": "X", "team_id": s.bi}).
		Expect(t, http.StatusBadRequest, "not_a_template")
	o := s.objective(t, s.admin, map[string]any{"scope": "organizacion"})
	s.admin.Post("/api/objectives/"+o.ID.String()+"/projects", map[string]any{"id": tpl.ID}).Expect(t, http.StatusBadRequest, "not_a_project")
	s.admin.Post("/api/objectives/"+o.ID.String()+"/projects", map[string]any{"id": bis.ID}).Expect(t, http.StatusOK)
	s.admin.Post("/api/projects/"+bis.ID.String()+"/template", nil).Expect(t, http.StatusConflict, "template_has_objectives")
}
