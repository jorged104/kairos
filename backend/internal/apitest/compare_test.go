package apitest

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

type compareDTO struct {
	Before   baselineDTO   `json:"before"`
	After    *baselineDTO  `json:"after"`
	Versions []baselineDTO `json:"versions"`
	Summary  struct {
		Added     int     `json:"added"`
		Removed   int     `json:"removed"`
		Moved     int     `json:"moved"`
		Unchanged int     `json:"unchanged"`
		BeforeEnd *string `json:"before_end"`
		AfterEnd  *string `json:"after_end"`
		EndShift  *int    `json:"end_shift"`
	} `json:"summary"`
	Items []struct {
		ID        *uuid.UUID `json:"id"`
		Key       string     `json:"key"`
		Title     string     `json:"title"`
		Change    string     `json:"change"`
		BeforeDue *string    `json:"before_due"`
		AfterDue  *string    `json:"after_due"`
		DueShift  *int       `json:"due_shift"`
		Deleted   bool       `json:"deleted"`
		Elsewhere bool       `json:"elsewhere"`
		Hidden    bool       `json:"hidden"`
	} `json:"items"`
}

func (s *workSetup) compare(t *testing.T, c *Client, project uuid.UUID, query string) compareDTO {
	t.Helper()
	var out compareDTO
	c.Get("/api/projects/"+project.String()+"/baseline/compare"+query).Expect(t, http.StatusOK).JSON(t, &out)
	return out
}

func TestCompareBaselines(t *testing.T) {
	s := newWorkSetup(t)
	p := s.project(t, s.lead, map[string]any{"identifier": "CMP", "name": "Comparar"})
	path := "/api/projects/" + p.ID.String()
	// Without a baseline there is nothing to compare (RF-1).
	s.viewer.Get(path+"/baseline/compare").Expect(t, http.StatusNotFound, "not_found")

	late := s.item(t, s.member, p.ID, map[string]any{"title": "Se retrasa", "due_date": "2026-03-10"})
	gone := s.item(t, s.member, p.ID, map[string]any{"title": "Se borra", "due_date": "2026-03-05"})
	s.item(t, s.member, p.ID, map[string]any{"title": "Igual", "start_date": "2026-03-01", "due_date": "2026-03-04"})
	s.lead.Post(path+"/baseline", map[string]any{}).Expect(t, http.StatusCreated)

	s.patch(s.member, s.get(t, late.ID), map[string]any{"due_date": "2026-03-15"}).Expect(t, http.StatusOK)
	s.member.Do(http.MethodDelete, "/api/work-items/"+gone.ID.String(), nil).Expect(t, http.StatusNoContent)
	added := s.item(t, s.member, p.ID, map[string]any{"title": "Nueva"})

	// One version: against today (RF-4); the biggest delay first (RF-15).
	c := s.compare(t, s.viewer, p.ID, "")
	if c.Before.Version != 1 || c.After != nil {
		t.Fatalf("sides = %+v / %+v", c.Before, c.After)
	}
	if c.Summary.Added != 1 || c.Summary.Removed != 1 || c.Summary.Moved != 1 || c.Summary.Unchanged != 1 {
		t.Fatalf("summary = %+v (RF-9)", c.Summary)
	}
	if c.Summary.EndShift == nil || *c.Summary.EndShift != 5 || *c.Summary.AfterEnd != "2026-03-15" {
		t.Fatalf("end = %+v (RF-10)", c.Summary)
	}
	first := c.Items[0]
	if first.Change != "moved" || first.DueShift == nil || *first.DueShift != 5 || first.ID == nil || *first.ID != late.ID {
		t.Fatalf("first = %+v", first)
	}
	for _, it := range c.Items {
		switch it.Change {
		case "removed": // RF-19, RF-25
			if !it.Deleted || it.ID != nil || it.Title != "Se borra" || it.Key != "CMP-2" {
				t.Fatalf("deleted = %+v", it)
			}
		case "added":
			if it.ID == nil || *it.ID != added.ID || it.BeforeDue != nil {
				t.Fatalf("added = %+v", it)
			}
		}
	}
	// The deleted work item does not count for the plan (RF-26).
	if o := s.overview(t, s.viewer, p.ID, "2026-03-06"); o.Figures.Plan != 50 {
		t.Fatalf("plan = %+v", o.Figures)
	}

	// Two versions: the previous one against the current one (RF-3), whatever
	// the order chosen (RF-5); the same on both sides is refused (RF-6).
	s.lead.Post(path+"/baseline", map[string]any{"reason": "Retraso"}).Expect(t, http.StatusCreated)
	c = s.compare(t, s.viewer, p.ID, "")
	if c.Before.Version != 1 || c.After == nil || c.After.Version != 2 || len(c.Versions) != 2 {
		t.Fatalf("default = %+v / %+v", c.Before, c.After)
	}
	c = s.compare(t, s.viewer, p.ID, "?from=2&to=1")
	if c.Before.Version != 1 || c.After.Version != 2 {
		t.Fatalf("swapped = %+v / %+v", c.Before, c.After)
	}
	c = s.compare(t, s.viewer, p.ID, "?from=today&to=2")
	if c.Before.Version != 2 || c.After != nil || c.Summary.Moved+c.Summary.Added+c.Summary.Removed != 0 {
		t.Fatalf("v2 vs today = %+v", c.Summary)
	}
	s.viewer.Get(path+"/baseline/compare?from=2&to=2").Expect(t, http.StatusBadRequest, "validation_failed")
	s.viewer.Get(path+"/baseline/compare?from=1&to=9").Expect(t, http.StatusNotFound, "not_found")

	// Who does not see the project, does not see it exists (RF-28).
	s.outsider.Get(path+"/baseline/compare").Expect(t, http.StatusNotFound, "not_found")
}

func TestCompareElsewhereAndOldBaselines(t *testing.T) {
	s := newWorkSetup(t)
	p := s.project(t, s.lead, map[string]any{"identifier": "CMP", "name": "Comparar"})
	other := s.project(t, s.admin, map[string]any{"identifier": "AJE", "name": "Ajeno", "team_id": s.other})
	away := s.item(t, s.member, p.ID, map[string]any{"title": "Se va"})
	old := s.item(t, s.member, p.ID, map[string]any{"title": "Antigua"})
	s.lead.Post("/api/projects/"+p.ID.String()+"/baseline", map[string]any{}).Expect(t, http.StatusCreated)

	db := s.env.Store.Pool
	// A work item now in a project the member does not see: its saved key and
	// title, without link (RF-20, RF-21).
	if _, err := db.Exec(t.Context(), "UPDATE work_items SET project_id = $1, sequence_number = 99 WHERE id = $2", other.ID, away.ID); err != nil {
		t.Fatal(err)
	}
	// A baseline from before spec 024 kept no title: once deleted, it is gone.
	if _, err := db.Exec(t.Context(), "UPDATE project_baseline_items SET item_key = NULL, title = NULL WHERE work_item_id = $1", old.ID); err != nil {
		t.Fatal(err)
	}
	s.member.Do(http.MethodDelete, "/api/work-items/"+old.ID.String(), nil).Expect(t, http.StatusNoContent)

	c := s.compare(t, s.member, p.ID, "")
	if len(c.Items) != 1 {
		t.Fatalf("items = %+v", c.Items)
	}
	if it := c.Items[0]; it.Change != "removed" || !it.Elsewhere || !it.Hidden || it.ID != nil || it.Title != "Se va" || it.Key != "CMP-1" {
		t.Fatalf("elsewhere = %+v", it)
	}
	// Who sees the other project gets the current key and the link.
	c = s.compare(t, s.admin, p.ID, "")
	if it := c.Items[0]; it.Hidden || it.ID == nil || it.Key != "AJE-99" {
		t.Fatalf("admin = %+v", it)
	}
}
