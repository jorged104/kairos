package apitest

import (
	"net/http"
	"slices"
	"testing"

	"github.com/google/uuid"
)

func (s *workSetup) label(t *testing.T, name string) uuid.UUID {
	t.Helper()
	var l struct {
		ID uuid.UUID `json:"id"`
	}
	s.admin.Post("/api/labels", map[string]any{"name": name, "color": "#22c55e"}).Expect(t, http.StatusCreated).JSON(t, &l)
	return l.ID
}

func (s *workSetup) labelsOf(t *testing.T, id uuid.UUID) []uuid.UUID {
	t.Helper()
	var wi struct {
		LabelIDs []uuid.UUID `json:"label_ids"`
	}
	s.admin.Get("/api/work-items/"+id.String()).Expect(t, http.StatusOK).JSON(t, &wi)
	return wi.LabelIDs
}

func TestLabelsCatalog(t *testing.T) {
	s := newWorkSetup(t)

	// Only admins manage them; names are unique among the active ones (RF-36, RF-37).
	s.lead.Post("/api/labels", map[string]any{"name": "Frontend", "color": "#22c55e"}).Expect(t, http.StatusForbidden)
	front := s.label(t, "Frontend")
	s.admin.Post("/api/labels", map[string]any{"name": " frontend ", "color": "#000000"}).Expect(t, http.StatusConflict, "catalog_name_conflict")
	s.admin.Post("/api/labels", map[string]any{"name": "Mal", "color": "verde"}).Expect(t, http.StatusBadRequest, "validation_failed")
	var l struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	s.admin.Do(http.MethodPatch, "/api/labels/"+front.String(), map[string]any{"name": "Front", "color": "#3B82F6", "position": 2}).
		Expect(t, http.StatusOK).JSON(t, &l)
	if l.Name != "Front" || l.Color != "#3b82f6" {
		t.Fatalf("label after patch = %+v", l)
	}
	s.admin.Post("/api/labels/"+front.String()+"/retire", nil).Expect(t, http.StatusOK)
	// A retired name can be used again.
	s.label(t, "Front")
	if got := s.env.AuditActions(front.String()); len(got) != 3 {
		t.Fatalf("audit = %v, want created, updated, retired", got)
	}

	// Everyone sees them in the catalog, shared by all projects (RF-38).
	var cat struct {
		Labels []struct {
			Name    string `json:"name"`
			Retired bool   `json:"retired"`
		} `json:"labels"`
	}
	s.member.Get("/api/catalog").Expect(t, http.StatusOK).JSON(t, &cat)
	if len(cat.Labels) != 2 {
		t.Fatalf("labels = %+v, want 2", cat.Labels)
	}
}

func TestWorkItemLabels(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	front, back, old := s.label(t, "Frontend"), s.label(t, "Backend"), s.label(t, "Viejo")

	// Created with labels, without repeats (RF-40).
	wi := s.item(t, s.member, project, map[string]any{"label_ids": []uuid.UUID{front, back, front}})
	if got := s.labelsOf(t, wi.ID); len(got) != 2 {
		t.Fatalf("labels = %v, want 2", got)
	}
	// A client that doesn't send them keeps them.
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"title": "Otro"}).Expect(t, http.StatusOK)
	if got := s.labelsOf(t, wi.ID); len(got) != 2 {
		t.Fatalf("labels after a patch without them = %v", got)
	}

	// A retired label stays where it is and can't be added (RF-39).
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"label_ids": []uuid.UUID{front, old}}).Expect(t, http.StatusOK)
	s.admin.Post("/api/labels/"+old.String()+"/retire", nil).Expect(t, http.StatusOK)
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"title": "Sigue", "label_ids": []uuid.UUID{front, old}}).Expect(t, http.StatusOK)
	other := s.item(t, s.member, project, map[string]any{})
	s.patch(s.member, s.get(t, other.ID), map[string]any{"label_ids": []uuid.UUID{old}}).Expect(t, http.StatusBadRequest, "label_retired")
	s.patch(s.member, s.get(t, other.ID), map[string]any{"label_ids": []uuid.UUID{uuid.New()}}).Expect(t, http.StatusBadRequest, "label_retired")

	// At most 10 (RF-41).
	many := make([]uuid.UUID, 11)
	for i := range many {
		many[i] = s.label(t, "L"+uuid.NewString()[:8])
	}
	s.patch(s.member, s.get(t, other.ID), map[string]any{"label_ids": many}).Expect(t, http.StatusBadRequest, "too_many_labels")
	s.patch(s.member, s.get(t, other.ID), map[string]any{"label_ids": many[:10]}).Expect(t, http.StatusOK)

	// Only who can modify the work item changes them (RF-40).
	s.patch(s.viewer, s.get(t, wi.ID), map[string]any{"label_ids": []uuid.UUID{}}).Expect(t, http.StatusForbidden)

	// Audited one by one (RF-46).
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"label_ids": []uuid.UUID{back}}).Expect(t, http.StatusOK)
	var after map[string]any
	if err := s.env.Store.Pool.QueryRow(t.Context(),
		"SELECT after FROM audit_log WHERE target_id = $1 AND action = 'work_item.updated' ORDER BY id DESC LIMIT 1", wi.ID.String()).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if len(after["labels_added"].([]any)) != 1 || len(after["labels_removed"].([]any)) != 2 {
		t.Fatalf("audit = %v, want 1 added and 2 removed", after)
	}

	// The tree shows them and the list filters by any of them (RF-43, RF-44).
	var tree struct {
		Items []struct {
			ID       uuid.UUID   `json:"id"`
			LabelIDs []uuid.UUID `json:"label_ids"`
		} `json:"items"`
	}
	s.member.Get("/api/projects/"+project.String()+"/tree").Expect(t, http.StatusOK).JSON(t, &tree)
	for _, n := range tree.Items {
		if n.ID == wi.ID && !slices.Equal(n.LabelIDs, []uuid.UUID{back}) {
			t.Fatalf("tree labels = %v, want backend", n.LabelIDs)
		}
	}
	list := func(q string) int {
		var page struct {
			Items []struct{} `json:"items"`
		}
		s.member.Get("/api/projects/"+project.String()+"/work-items?"+q).Expect(t, http.StatusOK).JSON(t, &page)
		return len(page.Items)
	}
	if n := list("label_id=" + back.String()); n != 1 {
		t.Fatalf("filter by backend = %d, want 1", n)
	}
	if n := list("label_id=" + back.String() + "&label_id=" + many[0].String()); n != 2 {
		t.Fatalf("filter by backend or another = %d, want 2", n)
	}
	if n := list("label_id=" + front.String()); n != 0 {
		t.Fatalf("filter by frontend = %d, want 0", n)
	}
	s.member.Get("/api/projects/"+project.String()+"/work-items?label_id=x").Expect(t, http.StatusBadRequest, "invalid_filter")
}
