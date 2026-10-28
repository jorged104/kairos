package apitest

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// weighted reads the weight and the derived ratio of a work item.
func (s *workSetup) weighted(t *testing.T, id uuid.UUID) (weight, ratio int) {
	t.Helper()
	var wi struct {
		Weight  int `json:"weight"`
		Derived struct {
			DoneRatio int `json:"done_ratio"`
		} `json:"derived"`
	}
	s.admin.Get("/api/work-items/"+id.String()).Expect(t, http.StatusOK).JSON(t, &wi)
	return wi.Weight, wi.Derived.DoneRatio
}

func TestTypeDefaultWeight(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	bug := s.env.CatalogID("work_item_types", "Bug")
	act := s.env.CatalogID("work_item_types", "Actividad")

	// New types weigh 1 unless told otherwise; out of range is rejected (RF-150, RF-152).
	var typ struct {
		ID            uuid.UUID `json:"id"`
		DefaultWeight int       `json:"default_weight"`
	}
	s.admin.Post("/api/work-item-types", map[string]any{"name": "Riesgo", "color": "#abcdef"}).Expect(t, http.StatusCreated).JSON(t, &typ)
	if typ.DefaultWeight != 1 {
		t.Fatalf("default weight of a new type = %d, want 1", typ.DefaultWeight)
	}
	s.admin.Post("/api/work-item-types", map[string]any{"name": "Seguimiento", "color": "#abcdef", "default_weight": 0}).
		Expect(t, http.StatusCreated).JSON(t, &typ)
	if typ.DefaultWeight != 0 {
		t.Fatalf("default weight = %d, want 0", typ.DefaultWeight)
	}
	s.admin.Post("/api/work-item-types", map[string]any{"name": "Grande", "color": "#abcdef", "default_weight": 101}).
		Expect(t, http.StatusBadRequest, "validation_failed")
	s.admin.Do(http.MethodPatch, "/api/work-item-types/"+bug.String(), map[string]any{"name": "Bug", "color": "#ef4444", "position": 6, "default_weight": 3}).
		Expect(t, http.StatusOK)
	// A client that doesn't send it keeps the current value.
	s.admin.Do(http.MethodPatch, "/api/work-item-types/"+bug.String(), map[string]any{"name": "Bug", "color": "#ef4444", "position": 6}).
		Expect(t, http.StatusOK).JSON(t, &typ)
	if typ.DefaultWeight != 3 {
		t.Fatalf("default weight after a patch without it = %d, want 3", typ.DefaultWeight)
	}

	// The type's weight is copied on creation (RF-151).
	wi := s.item(t, s.member, project, map[string]any{"type_id": bug})
	if w, _ := s.weighted(t, wi.ID); w != 3 {
		t.Fatalf("weight of a new bug = %d, want 3", w)
	}
	explicit := s.item(t, s.member, project, map[string]any{"type_id": bug, "weight": 8})
	if w, _ := s.weighted(t, explicit.ID); w != 8 {
		t.Fatalf("explicit weight = %d, want 8", w)
	}
	// Changing the type's weight, or the work item's type, leaves it alone.
	s.admin.Do(http.MethodPatch, "/api/work-item-types/"+bug.String(), map[string]any{"name": "Bug", "color": "#ef4444", "position": 6, "default_weight": 5}).
		Expect(t, http.StatusOK)
	if w, _ := s.weighted(t, wi.ID); w != 3 {
		t.Fatalf("weight after changing the type's default = %d, want 3", w)
	}
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"type_id": act}).Expect(t, http.StatusOK)
	if w, _ := s.weighted(t, wi.ID); w != 3 {
		t.Fatalf("weight after changing the type = %d, want 3", w)
	}

	// Bounds of a work item's weight (RF-149, RF-152).
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"weight": 0}).Expect(t, http.StatusOK)
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"weight": 100}).Expect(t, http.StatusOK)
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"weight": 101}).Expect(t, http.StatusBadRequest, "field_out_of_range")
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"weight": -1}).Expect(t, http.StatusBadRequest, "field_out_of_range")
	s.member.Post("/api/projects/"+project.String()+"/work-items", map[string]any{"type_id": bug, "title": "x", "weight": 101}).
		Expect(t, http.StatusBadRequest, "field_out_of_range")
}

func TestWeightedProgress(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	closed := s.env.CatalogID("statuses", "Cerrado")

	// epic (1, 0 %) > a (3, 50 %), b (1), tracking (0, 8 h).
	epic := s.item(t, s.member, project, map[string]any{"title": "Épica"})
	a := s.item(t, s.member, project, map[string]any{"title": "A", "parent_id": epic.ID, "weight": 3, "done_ratio": 50})
	b := s.item(t, s.member, project, map[string]any{"title": "B", "parent_id": epic.ID})
	tracking := s.item(t, s.member, project, map[string]any{"title": "Seguimiento", "parent_id": epic.ID, "weight": 0,
		"estimated_hours": 8, "remaining_hours": 8})

	// (1*0 + 3*50 + 1*0) / 5 = 30 (RF-153); the tracking item adds hours (RF-159).
	if _, r := s.weighted(t, epic.ID); r != 30 {
		t.Fatalf("epic ratio = %d, want 30", r)
	}
	if got := s.get(t, epic.ID).Derived; got.EstimatedHours != 8 {
		t.Fatalf("epic hours = %+v, want the tracking item's 8 h", got)
	}
	// Closing the tracking item doesn't move the parent (RF-149).
	s.patch(s.member, tracking, map[string]any{"status_id": closed}).Expect(t, http.StatusOK)
	if _, r := s.weighted(t, epic.ID); r != 30 {
		t.Fatalf("epic ratio after closing the tracking item = %d, want 30", r)
	}
	// Closing b counts 100 with its weight: (150 + 100) / 5 = 50.
	s.patch(s.member, b, map[string]any{"status_id": closed}).Expect(t, http.StatusOK)
	if _, r := s.weighted(t, epic.ID); r != 50 {
		t.Fatalf("epic ratio after closing b = %d, want 50", r)
	}
	// Changing a weight refreshes the ancestors (RF-158): (1*0 + 1*50 + 1*100) / 3 = 50 → a at 1.
	s.patch(s.member, s.get(t, a.ID), map[string]any{"weight": 1}).Expect(t, http.StatusOK)
	if _, r := s.weighted(t, epic.ID); r != 50 {
		t.Fatalf("epic ratio after changing a's weight = %d, want 50", r)
	}
	s.patch(s.member, s.get(t, a.ID), map[string]any{"weight": 7}).Expect(t, http.StatusOK)
	// (1*0 + 7*50 + 1*100) / 9 = 50.
	if _, r := s.weighted(t, epic.ID); r != 50 {
		t.Fatalf("epic ratio with a at 7 = %d, want 50", r)
	}
	// Weight 0 everywhere falls back to the simple mean (RF-153).
	lone := s.item(t, s.member, project, map[string]any{"title": "Solo", "weight": 0, "done_ratio": 40})
	if _, r := s.weighted(t, lone.ID); r != 40 {
		t.Fatalf("ratio of a lone tracking item = %d, want 40", r)
	}
	if !t.Failed() {
		verifyAggregates(t, s.env, project)
	}

	// Milestones weigh their effective work items (RF-154).
	m := s.milestone(t, project, map[string]any{"name": "M1"})
	s.link(t, s.lead, m.ID, epic.ID).Expect(t, http.StatusNoContent)
	var p struct {
		Progress struct {
			Total        int   `json:"total"`
			Closed       int   `json:"closed"`
			Percent      int   `json:"percent"`
			Weight       int64 `json:"weight"`
			WeightClosed int64 `json:"weight_closed"`
		} `json:"progress"`
	}
	s.viewer.Get("/api/milestones/"+m.ID.String()).Expect(t, http.StatusOK).JSON(t, &p)
	// epic 1 + a 7 + b 1 (closed) + tracking 0 (closed): 1 of 9 closed → 11 %; counts include all four.
	if got := p.Progress; got.Total != 4 || got.Closed != 2 || got.Weight != 9 || got.WeightClosed != 1 || got.Percent != 11 {
		t.Fatalf("milestone progress = %+v", got)
	}
	// Only tracking items: 0 % with the counts intact.
	only := s.milestone(t, project, map[string]any{"name": "M2"})
	s.link(t, s.lead, only.ID, tracking.ID).Expect(t, http.StatusNoContent)
	s.viewer.Get("/api/milestones/"+only.ID.String()).Expect(t, http.StatusOK).JSON(t, &p)
	if got := p.Progress; got.Total != 1 || got.Closed != 1 || got.Percent != 0 {
		t.Fatalf("progress of a tracking-only milestone = %+v", got)
	}

	// The closing event carries the weight (RF-155).
	var evs struct {
		Items []struct {
			Type    string         `json:"type"`
			Payload map[string]any `json:"payload"`
		} `json:"items"`
	}
	s.admin.Get("/api/events?subject_id="+b.ID.String()).Expect(t, http.StatusOK).JSON(t, &evs)
	if len(evs.Items) != 1 || evs.Items[0].Type != "work_item.closed" || evs.Items[0].Payload["weight"] != float64(1) {
		t.Fatalf("events of b = %+v", evs.Items)
	}
}
