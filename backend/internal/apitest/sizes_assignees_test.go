package apitest

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// sizedDTO is what the spec 006 tests read of a work item.
type sizedDTO struct {
	ID        uuid.UUID               `json:"id"`
	Version   int32                   `json:"version"`
	Weight    int                     `json:"weight"`
	SizeID    *uuid.UUID              `json:"size_id"`
	Assignee  *struct{ ID uuid.UUID } `json:"assignee"`
	Assignees []struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	} `json:"assignees"`
}

func (s *workSetup) sized(t *testing.T, id uuid.UUID) sizedDTO {
	t.Helper()
	var wi sizedDTO
	s.admin.Get("/api/work-items/"+id.String()).Expect(t, http.StatusOK).JSON(t, &wi)
	return wi
}

func ids(d sizedDTO) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(d.Assignees))
	for _, a := range d.Assignees {
		out = append(out, a.ID)
	}
	return out
}

func TestSizesCatalog(t *testing.T) {
	s := newWorkSetup(t)
	var cat struct {
		Sizes []struct {
			ID      uuid.UUID `json:"id"`
			Name    string    `json:"name"`
			Retired bool      `json:"retired"`
		} `json:"sizes"`
	}
	s.member.Get("/api/catalog").Expect(t, http.StatusOK).JSON(t, &cat)
	// XS to XL, in that order (RF-2).
	var names []string
	for _, sz := range cat.Sizes {
		names = append(names, sz.Name)
	}
	if strings.Join(names, ",") != "XS,S,M,L,XL" {
		t.Fatalf("sizes = %v, want XS to XL", names)
	}

	// Only admins manage them; names are unique and they carry no weight (RF-1).
	s.lead.Post("/api/work-item-sizes", map[string]any{"name": "XXL"}).Expect(t, http.StatusForbidden)
	var xxl map[string]any
	s.admin.Post("/api/work-item-sizes", map[string]any{"name": "XXL", "position": 6}).Expect(t, http.StatusCreated).JSON(t, &xxl)
	if _, ok := xxl["weight"]; ok {
		t.Fatalf("size = %v, want no weight", xxl)
	}
	id := xxl["id"].(string)
	s.admin.Post("/api/work-item-sizes", map[string]any{"name": " xxl "}).Expect(t, http.StatusConflict, "catalog_name_conflict")
	s.admin.Do(http.MethodPatch, "/api/work-item-sizes/"+id, map[string]any{"name": "XXXL", "position": 6}).
		Expect(t, http.StatusOK).JSON(t, &xxl)
	if xxl["name"] != "XXXL" {
		t.Fatalf("name after patch = %v, want XXXL", xxl["name"])
	}
	s.admin.Post("/api/work-item-sizes/"+id+"/retire", nil).Expect(t, http.StatusOK)
	if got := s.env.AuditActions(id); len(got) != 3 {
		t.Fatalf("audit = %v, want created, updated, retired", got)
	}
}

func TestWorkItemSize(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	m, l := s.env.CatalogID("work_item_sizes", "M"), s.env.CatalogID("work_item_sizes", "L")

	// The size doesn't touch the weight: the type's default stays (RF-5).
	wi := s.item(t, s.member, project, map[string]any{"size_id": m})
	if got := s.sized(t, wi.ID); got.Weight != 1 || got.SizeID == nil || *got.SizeID != m {
		t.Fatalf("created with M = %+v, want weight 1", got)
	}
	if got := s.sized(t, s.item(t, s.member, project, map[string]any{"size_id": m, "weight": 7}).ID); got.Weight != 7 {
		t.Fatalf("created with M and weight 7 = %d", got.Weight)
	}

	// Changing the size keeps the weight sent (RF-5, RF-6).
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"weight": 9}).Expect(t, http.StatusOK)
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"size_id": l}).Expect(t, http.StatusOK)
	if got := s.sized(t, wi.ID); got.Weight != 9 || *got.SizeID != l {
		t.Fatalf("after L = %+v, want 9 with L", got)
	}
	// A client that doesn't send the size keeps it.
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"title": "Otra"}).Expect(t, http.StatusOK)
	if got := s.sized(t, wi.ID); got.SizeID == nil || *got.SizeID != l {
		t.Fatalf("size after a patch without it = %v, want L", got.SizeID)
	}
	// Removing the size keeps the weight (RF-7).
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"size_id": nil}).Expect(t, http.StatusOK)
	if got := s.sized(t, wi.ID); got.SizeID != nil || got.Weight != 9 {
		t.Fatalf("after removing the size = %+v, want none and 9", got)
	}

	s.patch(s.member, s.get(t, wi.ID), map[string]any{"size_id": m}).Expect(t, http.StatusOK)

	// A retired size stays where it is and can't be chosen (RF-9).
	s.admin.Post("/api/work-item-sizes/"+l.String()+"/retire", nil).Expect(t, http.StatusOK)
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"size_id": l}).Expect(t, http.StatusBadRequest, "size_retired")
	s.member.Post("/api/projects/"+project.String()+"/work-items", map[string]any{
		"type_id": s.env.CatalogID("work_item_types", "Actividad"), "title": "x", "size_id": l,
	}).Expect(t, http.StatusBadRequest, "size_retired")
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"size_id": uuid.New()}).Expect(t, http.StatusBadRequest, "validation_failed")

	// The tree shows it (RF-56).
	var tree struct {
		Items []struct {
			ID     uuid.UUID  `json:"id"`
			SizeID *uuid.UUID `json:"size_id"`
		} `json:"items"`
	}
	s.member.Get("/api/projects/"+project.String()+"/tree").Expect(t, http.StatusOK).JSON(t, &tree)
	for _, n := range tree.Items {
		if n.ID == wi.ID && (n.SizeID == nil || *n.SizeID != m) {
			t.Fatalf("tree size = %v, want M", n.SizeID)
		}
	}
}

func TestAssignees(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	ana := s.env.CreateUser(UserSpec{Name: "Ana", Email: "ana@x.com", Roles: []string{"member"}, Teams: []uuid.UUID{s.rep}})

	// A list, in its order, without repeats (RF-12); assignee is the first.
	wi := s.item(t, s.member, project, map[string]any{"assignee_ids": []uuid.UUID{s.memberID, ana, s.memberID}})
	got := s.sized(t, wi.ID)
	if l := ids(got); len(l) != 2 || l[0] != s.memberID || l[1] != ana || got.Assignee == nil || got.Assignee.ID != s.memberID {
		t.Fatalf("assignees = %+v", got)
	}
	// Viewers and inactive people can't be added (RF-14); the whole change is rejected.
	viewer := s.env.CreateUser(UserSpec{Email: "v2@x.com", Roles: []string{"viewer"}})
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"assignee_ids": []uuid.UUID{s.memberID, viewer}}).
		Expect(t, http.StatusBadRequest, "invalid_assignee")
	// At most 20 (RF-13).
	many := make([]uuid.UUID, 21)
	for i := range many {
		many[i] = s.env.CreateUser(UserSpec{Email: uuid.NewString()[:8] + "@x.com", Roles: []string{"member"}})
	}
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"assignee_ids": many}).Expect(t, http.StatusBadRequest, "too_many_assignees")
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"assignee_ids": many[:20]}).Expect(t, http.StatusOK)
	if n := len(s.sized(t, wi.ID).Assignees); n != 20 {
		t.Fatalf("assignees = %d, want 20", n)
	}

	// Someone deactivated stays (RF-15) but can't be added again elsewhere.
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"assignee_ids": []uuid.UUID{ana, s.memberID}}).Expect(t, http.StatusOK)
	if _, err := s.env.Store.Pool.Exec(context.Background(), "UPDATE users SET status = 'inactive' WHERE id = $1", ana); err != nil {
		t.Fatal(err)
	}
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"title": "Sigue", "assignee_ids": []uuid.UUID{ana, s.memberID}}).Expect(t, http.StatusOK)
	other := s.item(t, s.member, project, map[string]any{})
	s.patch(s.member, s.get(t, other.ID), map[string]any{"assignee_ids": []uuid.UUID{ana}}).Expect(t, http.StatusBadRequest, "invalid_assignee")

	// A previous client sending only the first assignee keeps the list; another one replaces it.
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"title": "Viejo"}).Expect(t, http.StatusOK)
	if l := ids(s.sized(t, wi.ID)); len(l) != 2 {
		t.Fatalf("list after an old client's patch = %v, want 2", l)
	}
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"assignee_id": s.memberID}).Expect(t, http.StatusOK)
	if l := ids(s.sized(t, wi.ID)); len(l) != 1 || l[0] != s.memberID {
		t.Fatalf("list after an old client changed the assignee = %v", l)
	}
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"assignee_ids": []uuid.UUID{s.leadID, s.memberID}}).Expect(t, http.StatusOK)

	// The filter finds it by any of them (RF-19).
	list := func(q string) []uuid.UUID {
		var page struct {
			Items []struct {
				ID        uuid.UUID `json:"id"`
				Assignees []struct {
					ID uuid.UUID `json:"id"`
				} `json:"assignees"`
			} `json:"items"`
		}
		s.member.Get("/api/projects/"+project.String()+"/work-items?"+q).Expect(t, http.StatusOK).JSON(t, &page)
		out := []uuid.UUID{}
		for _, it := range page.Items {
			out = append(out, it.ID)
			if it.ID == wi.ID && len(it.Assignees) != 2 {
				t.Fatalf("listed assignees = %+v, want 2", it.Assignees)
			}
		}
		return out
	}
	if got := list("assignee_id=" + s.memberID.String()); len(got) != 1 || got[0] != wi.ID {
		t.Fatalf("by member = %v", got)
	}
	if got := list("assignee_id=" + s.leadID.String()); len(got) != 1 || got[0] != wi.ID {
		t.Fatalf("by lead = %v", got)
	}
	if got := list("assignee_id=none"); len(got) != 1 || got[0] != other.ID {
		t.Fatalf("unassigned = %v", got)
	}

	// Events carry all of them (RF-20) and the audit who came and left (RF-21).
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"status_id": s.env.CatalogID("statuses", "Cerrado")}).Expect(t, http.StatusOK)
	var evs struct {
		Items []struct {
			Type    string         `json:"type"`
			Payload map[string]any `json:"payload"`
		} `json:"items"`
	}
	s.admin.Get("/api/events?subject_id="+wi.ID.String()).Expect(t, http.StatusOK).JSON(t, &evs)
	if len(evs.Items) == 0 {
		t.Fatal("no closed event")
	}
	if got, _ := evs.Items[0].Payload["assignee_ids"].([]any); len(got) != 2 || got[0] != s.leadID.String() {
		t.Fatalf("event assignees = %v (RF-20)", evs.Items[0].Payload)
	}
	var audit int
	if err := s.env.Store.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_log WHERE target_id = $1 AND after ? 'assignees_added'`, wi.ID.String()).Scan(&audit); err != nil {
		t.Fatal(err)
	}
	if audit == 0 {
		t.Fatal("assignee changes not audited (RF-21)")
	}
}

// TestAssigneeCompatibility checks the trigger that keeps the list in step with
// what the previous API writes, assignee_id only (RF-16).
func TestAssigneeCompatibility(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	wi := s.item(t, s.member, project, map[string]any{"assignee_ids": []uuid.UUID{s.memberID, s.leadID}})
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := s.env.Store.Pool.Exec(context.Background(), sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	// The same first person: the list stays.
	exec("UPDATE work_items SET assignee_id = $2 WHERE id = $1", wi.ID, s.memberID)
	if l := ids(s.sized(t, wi.ID)); len(l) != 2 {
		t.Fatalf("list = %v, want 2", l)
	}
	// Another person replaces it; null empties it.
	exec("UPDATE work_items SET assignee_id = $2 WHERE id = $1", wi.ID, s.adminID)
	if l := ids(s.sized(t, wi.ID)); len(l) != 1 || l[0] != s.adminID {
		t.Fatalf("list = %v, want admin", l)
	}
	exec("UPDATE work_items SET assignee_id = NULL WHERE id = $1", wi.ID)
	if l := ids(s.sized(t, wi.ID)); len(l) != 0 {
		t.Fatalf("list = %v, want empty", l)
	}
}
