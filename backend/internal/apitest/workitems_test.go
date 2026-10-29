package apitest

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

type workItemDTO struct {
	ID               uuid.UUID   `json:"id"`
	Key              string      `json:"key"`
	SequenceNumber   int         `json:"sequence_number"`
	TypeID           uuid.UUID   `json:"type_id"`
	StatusID         uuid.UUID   `json:"status_id"`
	PriorityID       uuid.UUID   `json:"priority_id"`
	Title            string      `json:"title"`
	Version          int32       `json:"version"`
	DoneRatio        int         `json:"done_ratio"`
	RemainingHours   *float64    `json:"remaining_hours"`
	EstimatedHours   *float64    `json:"estimated_hours"`
	Depth            int         `json:"depth"`
	ClosedAt         *string     `json:"closed_at"`
	AllowedStatusIDs []uuid.UUID `json:"allowed_status_ids"`
	Parent           *struct {
		ID  uuid.UUID `json:"id"`
		Key string    `json:"key"`
	} `json:"parent"`
	Assignee *struct {
		ID uuid.UUID `json:"id"`
	} `json:"assignee"`
	Derived struct {
		EstimatedHours float64 `json:"estimated_hours"`
		RemainingHours float64 `json:"remaining_hours"`
		DoneRatio      int     `json:"done_ratio"`
	} `json:"derived"`
	Dependencies []struct {
		ID            uuid.UUID `json:"id"`
		Type          string    `json:"type"`
		Direction     string    `json:"direction"`
		DatesViolated bool      `json:"dates_violated"`
		Other         struct {
			Key string `json:"key"`
		} `json:"other"`
	} `json:"dependencies"`
	Can struct {
		Write  bool `json:"write"`
		Delete bool `json:"delete"`
	} `json:"can"`
}

// workProject creates project KAI of team BI and returns its id.
func (s *workSetup) workProject(t *testing.T) uuid.UUID {
	t.Helper()
	return s.project(t, s.lead, map[string]any{"identifier": "KAI", "name": "Kairos"}).ID
}

func (s *workSetup) item(t *testing.T, c *Client, project uuid.UUID, body map[string]any) workItemDTO {
	t.Helper()
	if _, ok := body["type_id"]; !ok {
		body["type_id"] = s.env.CatalogID("work_item_types", "Actividad")
	}
	if _, ok := body["title"]; !ok {
		body["title"] = "Tarea"
	}
	var wi workItemDTO
	c.Post("/api/projects/"+project.String()+"/work-items", body).Expect(t, http.StatusCreated).JSON(t, &wi)
	return wi
}

func (s *workSetup) get(t *testing.T, id uuid.UUID) workItemDTO {
	t.Helper()
	var wi workItemDTO
	s.admin.Get("/api/work-items/"+id.String()).Expect(t, http.StatusOK).JSON(t, &wi)
	return wi
}

// patch sends the work item back with some changes.
func (s *workSetup) patch(c *Client, wi workItemDTO, changes map[string]any) *Response {
	body := map[string]any{
		"version": wi.Version, "type_id": wi.TypeID, "status_id": wi.StatusID, "priority_id": wi.PriorityID,
		"title": wi.Title, "done_ratio": wi.DoneRatio, "estimated_hours": wi.EstimatedHours, "remaining_hours": wi.RemainingHours,
	}
	if wi.Parent != nil {
		body["parent_id"] = wi.Parent.ID
	}
	if wi.Assignee != nil {
		body["assignee_id"] = wi.Assignee.ID
	}
	for k, v := range changes {
		body[k] = v
	}
	return c.Do(http.MethodPatch, "/api/work-items/"+wi.ID.String(), body)
}

func TestCreateWorkItem(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)

	wi := s.item(t, s.member, project, map[string]any{"title": "  Primera  ", "estimated_hours": 8, "assignee_id": s.memberID,
		"description": map[string]any{"type": "doc", "content": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "x"}}}}}})
	if wi.Key != "KAI-1" || wi.Title != "Primera" || wi.StatusID != s.env.CatalogID("statuses", "Nuevo") ||
		wi.PriorityID != s.env.CatalogID("priorities", "Normal") || !wi.Can.Write || !wi.Can.Delete {
		t.Fatalf("work item = %+v (RF-62, RF-64)", wi)
	}
	var byKey workItemDTO
	s.viewer.Get("/api/work-items/by-key/kai-1").Expect(t, http.StatusOK).JSON(t, &byKey)
	if byKey.ID != wi.ID || byKey.Can.Write {
		t.Fatalf("by key = %+v (RF-133)", byKey)
	}
	s.viewer.Get("/api/work-items/by-key/KAI-99").Expect(t, http.StatusNotFound)

	p := "/api/projects/" + project.String() + "/work-items"
	act := s.env.CatalogID("work_item_types", "Actividad")
	cases := []struct {
		body map[string]any
		code string
	}{
		{map[string]any{"type_id": act, "title": ""}, "validation_failed"},
		{map[string]any{"type_id": act, "title": string(make([]byte, 256))}, "validation_failed"},
		{map[string]any{"type_id": act, "title": "x", "start_date": "2026-02-01", "due_date": "2026-01-01"}, "invalid_date_range"},
		{map[string]any{"type_id": act, "title": "x", "estimated_hours": 10001}, "field_out_of_range"},
		{map[string]any{"type_id": act, "title": "x", "remaining_hours": -1}, "field_out_of_range"},
		{map[string]any{"type_id": act, "title": "x", "done_ratio": 101}, "field_out_of_range"},
		{map[string]any{"type_id": act, "title": "x", "assignee_id": s.env.CreateUser(UserSpec{Email: "vv@x.com", Roles: []string{"viewer"}})}, "invalid_assignee"},
		{map[string]any{"type_id": act, "title": "x", "status_id": s.env.CatalogID("statuses", "Cerrado")}, "validation_failed"},
		{map[string]any{"type_id": act, "title": "x", "description": map[string]any{"type": "doc", "content": []any{map[string]any{"type": "script"}}}}, "invalid_rich_text"},
	}
	for _, c := range cases {
		s.member.Post(p, c.body).Expect(t, http.StatusBadRequest, c.code)
	}

	// Type not enabled in the project (RF-65).
	var proj projectDTO
	s.lead.Get("/api/projects/"+project.String()).JSON(t, &proj)
	s.lead.Do(http.MethodPut, "/api/projects/"+project.String()+"/types", map[string]any{"ids": []uuid.UUID{act}}).Expect(t, http.StatusOK)
	s.member.Post(p, map[string]any{"type_id": s.env.CatalogID("work_item_types", "Bug"), "title": "x"}).Expect(t, http.StatusBadRequest, "type_not_enabled")

	// Who writes: participants yes; outsiders and viewers no.
	s.outsider.Post(p, map[string]any{"type_id": act, "title": "x"}).Expect(t, http.StatusForbidden, "forbidden")
	s.viewer.Post(p, map[string]any{"type_id": act, "title": "x"}).Expect(t, http.StatusForbidden, "forbidden")
}

func TestWorkItemNumbersUnderConcurrency(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	act := s.env.CatalogID("work_item_types", "Actividad")
	const n = 20
	var wg sync.WaitGroup
	keys := make(chan int, n)
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var wi workItemDTO
			res := s.member.Post("/api/projects/"+project.String()+"/work-items", map[string]any{"type_id": act, "title": "x"})
			if res.Status == http.StatusCreated {
				res.JSON(t, &wi)
				keys <- wi.SequenceNumber
			}
		}()
	}
	wg.Wait()
	close(keys)
	var got []int
	for k := range keys {
		got = append(got, k)
	}
	sort.Ints(got)
	if len(got) != n || got[0] != 1 || got[n-1] != n {
		t.Fatalf("numbers = %v, want 1..%d with no repeats (RF-63)", got, n)
	}
}

func TestWorkItemVersionConflict(t *testing.T) {
	s := newWorkSetup(t)
	wi := s.item(t, s.member, s.workProject(t), map[string]any{})
	s.patch(s.member, wi, map[string]any{"title": "Primero"}).Expect(t, http.StatusOK)
	res := s.patch(s.lead, wi, map[string]any{"title": "Segundo"}).Expect(t, http.StatusConflict, "version_conflict")
	if got := s.get(t, wi.ID); got.Title != "Primero" || got.Version != wi.Version+1 {
		t.Fatalf("after conflict = %+v; body %s (RF-73)", got, res.Body)
	}
}

func TestCompositionAndDerivedValues(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	epic := s.item(t, s.member, project, map[string]any{"title": "Épica", "estimated_hours": 10, "remaining_hours": 10})
	a := s.item(t, s.member, project, map[string]any{"title": "A", "parent_id": epic.ID, "estimated_hours": 20, "remaining_hours": 5, "done_ratio": 50})
	b := s.item(t, s.member, project, map[string]any{"title": "B", "parent_id": epic.ID, "estimated_hours": 10, "remaining_hours": 10})
	if a.Parent == nil || a.Parent.Key != epic.Key || a.Depth != 1 {
		t.Fatalf("child = %+v", a)
	}
	// epic: 40 h, remaining 25; ratio by weight, all 1: (0 + 50 + 0) / 3 = 17 (RF-80, RF-153).
	if got := s.get(t, epic.ID).Derived; got.EstimatedHours != 40 || got.RemainingHours != 25 || got.DoneRatio != 17 {
		t.Fatalf("derived = %+v", got)
	}

	// Closing B finishes it and moves the parent (RF-83, RF-84).
	closed := s.env.CatalogID("statuses", "Resuelto")
	var bClosed workItemDTO
	s.patch(s.member, b, map[string]any{"status_id": closed}).Expect(t, http.StatusOK).JSON(t, &bClosed)
	if bClosed.DoneRatio != 100 || bClosed.RemainingHours == nil || *bClosed.RemainingHours != 0 || bClosed.ClosedAt == nil {
		t.Fatalf("closed = %+v", bClosed)
	}
	if got := s.get(t, epic.ID).Derived; got.RemainingHours != 15 || got.DoneRatio != 50 {
		t.Fatalf("derived after close = %+v, want 15 h and (0+50+100)/3 = 50", got)
	}

	// Reopening B restarts it although the client sends 100 % and 0 h back (RF-160).
	var bReopened workItemDTO
	s.patch(s.lead, bClosed, map[string]any{"status_id": s.env.CatalogID("statuses", "En progreso")}).
		Expect(t, http.StatusOK).JSON(t, &bReopened)
	if bReopened.DoneRatio != 0 || bReopened.RemainingHours == nil || *bReopened.RemainingHours != 10 || bReopened.ClosedAt != nil {
		t.Fatalf("reopened = %+v, want 0 %% and the 10 estimated hours", bReopened)
	}
	if got := s.get(t, epic.ID).Derived; got.RemainingHours != 25 || got.DoneRatio != 17 {
		t.Fatalf("derived after reopen = %+v, want 25 h and 17", got)
	}

	// Moving A out of the epic updates both sides (RF-57, RF-83); the reopened B counts its 10 h again.
	s.patch(s.member, s.get(t, a.ID), map[string]any{"parent_id": nil}).Expect(t, http.StatusOK)
	if got := s.get(t, epic.ID).Derived; got.EstimatedHours != 20 || got.RemainingHours != 20 {
		t.Fatalf("derived after move = %+v", got)
	}
	if got := s.get(t, a.ID); got.Depth != 0 || got.Parent != nil {
		t.Fatalf("moved = %+v", got)
	}

	// Cross project, cycles and depth (RF-76 to RF-78).
	other := s.project(t, s.lead, map[string]any{"identifier": "OTR", "name": "Otro"}).ID
	s.member.Post("/api/projects/"+other.String()+"/work-items", map[string]any{
		"type_id": s.env.CatalogID("work_item_types", "Actividad"), "title": "x", "parent_id": epic.ID,
	}).Expect(t, http.StatusBadRequest, "cross_project")
	child := s.item(t, s.member, project, map[string]any{"parent_id": a.ID})
	s.patch(s.member, s.get(t, a.ID), map[string]any{"parent_id": child.ID}).Expect(t, http.StatusConflict, "work_item_cycle")
	s.patch(s.member, s.get(t, a.ID), map[string]any{"parent_id": a.ID}).Expect(t, http.StatusConflict, "work_item_cycle")

	parent := a.ID
	for i := 0; i < 9; i++ { // a is a root again: the chain reaches depth 9
		parent = s.item(t, s.member, project, map[string]any{"parent_id": parent}).ID
	}
	s.member.Post("/api/projects/"+project.String()+"/work-items", map[string]any{
		"type_id": s.env.CatalogID("work_item_types", "Actividad"), "title": "x", "parent_id": parent,
	}).Expect(t, http.StatusConflict, "work_item_too_deep")
	// The epic (with B below) cannot go under the deepest item either.
	s.patch(s.member, s.get(t, epic.ID), map[string]any{"parent_id": child.ID}).Expect(t, http.StatusOK)
	s.patch(s.member, s.get(t, epic.ID), map[string]any{"parent_id": parent}).Expect(t, http.StatusConflict, "work_item_too_deep")

	var anc struct {
		Items []struct{ Key string } `json:"items"`
	}
	s.viewer.Get("/api/work-items/"+parent.String()+"/ancestors").Expect(t, http.StatusOK).JSON(t, &anc)
	if len(anc.Items) != 9 || anc.Items[8].Key != a.Key {
		t.Fatalf("ancestors = %+v (RF-79)", anc.Items)
	}
	var desc struct {
		Items []struct{ Key string } `json:"items"`
	}
	s.viewer.Get("/api/work-items/"+a.ID.String()+"/descendants").Expect(t, http.StatusOK).JSON(t, &desc)
	if len(desc.Items) != 12 { // child, epic, B and the 9-level chain
		t.Fatalf("descendants = %d (RF-79)", len(desc.Items))
	}
	var tree struct {
		Items []struct{ Key string } `json:"items"`
	}
	s.viewer.Get("/api/projects/"+project.String()+"/tree").Expect(t, http.StatusOK).JSON(t, &tree)
	if len(tree.Items) != 13 {
		t.Fatalf("tree = %d items (RF-130)", len(tree.Items))
	}
	if !t.Failed() {
		verifyAggregates(t, s.env, project)
	}

	// Deleting: never with children; leaves yes, and the parent follows (RF-85).
	s.member.Do(http.MethodDelete, "/api/work-items/"+a.ID.String(), nil).Expect(t, http.StatusConflict, "has_children")
	s.outsider.Do(http.MethodDelete, "/api/work-items/"+b.ID.String(), nil).Expect(t, http.StatusForbidden)
	s.member.Do(http.MethodDelete, "/api/work-items/"+b.ID.String(), nil).Expect(t, http.StatusNoContent)
	if got := s.get(t, epic.ID).Derived; got.EstimatedHours != 10 {
		t.Fatalf("derived after delete = %+v", got)
	}
}

// verifyAggregates compares the stored aggregates with a full rebuild.
func verifyAggregates(t *testing.T, env *Env, project uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	snapshot := func() map[uuid.UUID][3]float64 {
		rows, err := env.Store.Pool.Query(ctx, "SELECT id, agg_estimated::float8, agg_remaining::float8, weighted_done_ratio FROM work_items WHERE project_id = $1", project)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := map[uuid.UUID][3]float64{}
		for rows.Next() {
			var id uuid.UUID
			var e, r float64
			var d int
			_ = rows.Scan(&id, &e, &r, &d)
			out[id] = [3]float64{e, r, float64(d)}
		}
		return out
	}
	before := snapshot()
	if _, err := env.Store.Pool.Exec(ctx, "SELECT work_item_recompute_project($1)", project); err != nil {
		t.Fatal(err)
	}
	after := snapshot()
	for id, v := range after {
		if before[id] != v {
			t.Fatalf("aggregates of %s = %v, a rebuild gives %v", id, before[id], v)
		}
	}
}

func TestConcurrentCrossMovesCannotBuildACycle(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	a := s.item(t, s.member, project, map[string]any{"title": "A"})
	b := s.item(t, s.member, project, map[string]any{"title": "B"})
	var wg sync.WaitGroup
	statuses := make([]int, 2)
	for i, pair := range [][2]workItemDTO{{a, b}, {b, a}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses[i] = s.patch(s.member, pair[0], map[string]any{"parent_id": pair[1].ID}).Status
		}()
	}
	wg.Wait()
	ok := 0
	for _, st := range statuses {
		if st == http.StatusOK {
			ok++
		}
	}
	if ok != 1 {
		t.Fatalf("statuses = %v, want exactly one move accepted (RF-77)", statuses)
	}
}

func TestWorkflowAndEvents(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	bug := s.env.CatalogID("work_item_types", "Bug")
	nuevo := s.env.CatalogID("statuses", "Nuevo")
	prog := s.env.CatalogID("statuses", "En progreso")
	cerrado := s.env.CatalogID("statuses", "Cerrado")
	resuelto := s.env.CatalogID("statuses", "Resuelto")
	s.admin.Do(http.MethodPut, "/api/work-item-types/"+bug.String()+"/transitions", map[string]any{"transitions": []map[string]any{
		{"from_status_id": nuevo, "to_status_id": prog},
		{"from_status_id": prog, "to_status_id": cerrado},
		{"from_status_id": cerrado, "to_status_id": resuelto},
		{"from_status_id": resuelto, "to_status_id": prog, "role": "lead"},
	}}).Expect(t, http.StatusOK)

	wi := s.item(t, s.member, project, map[string]any{"type_id": bug, "assignee_id": s.memberID})
	if len(wi.AllowedStatusIDs) != 1 || wi.AllowedStatusIDs[0] != prog {
		t.Fatalf("allowed = %v, want only En progreso (RF-89)", wi.AllowedStatusIDs)
	}
	s.patch(s.member, wi, map[string]any{"status_id": cerrado}).Expect(t, http.StatusConflict, "transition_not_allowed")
	wi = s.get(t, wi.ID)
	s.patch(s.member, wi, map[string]any{"status_id": prog}).Expect(t, http.StatusOK)
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"status_id": cerrado}).Expect(t, http.StatusOK)
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"status_id": resuelto}).Expect(t, http.StatusOK) // closed → closed
	// Reopening is only for leads; admins can force anything (RF-88).
	s.patch(s.member, s.get(t, wi.ID), map[string]any{"status_id": prog}).Expect(t, http.StatusConflict, "transition_not_allowed")
	s.patch(s.lead, s.get(t, wi.ID), map[string]any{"status_id": prog}).Expect(t, http.StatusOK)
	s.patch(s.admin, s.get(t, wi.ID), map[string]any{"status_id": cerrado}).Expect(t, http.StatusOK)

	var evs struct {
		Items []struct {
			Type    string         `json:"type"`
			Payload map[string]any `json:"payload"`
		} `json:"items"`
	}
	s.admin.Get("/api/events?subject_id="+wi.ID.String()).Expect(t, http.StatusOK).JSON(t, &evs)
	var types []string
	for i := len(evs.Items) - 1; i >= 0; i-- {
		types = append(types, evs.Items[i].Type)
	}
	want := []string{"work_item.closed", "work_item.reopened", "work_item.closed"}
	if len(types) != 3 || types[0] != want[0] || types[1] != want[1] || types[2] != want[2] {
		t.Fatalf("events = %v, want %v (RF-116 to RF-118)", types, want)
	}
	if evs.Items[0].Payload["assignee_id"] != s.memberID.String() || evs.Items[0].Payload["key"] != wi.Key {
		t.Fatalf("payload = %v (RF-122)", evs.Items[0].Payload)
	}
	s.member.Get("/api/events").Expect(t, http.StatusForbidden)

	// A type without transitions allows any change (RF-87).
	act := s.item(t, s.member, project, map[string]any{})
	s.patch(s.member, act, map[string]any{"status_id": cerrado}).Expect(t, http.StatusOK)
}

func TestDependencies(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	a := s.item(t, s.member, project, map[string]any{"title": "A", "due_date": "2026-03-10"})
	b := s.item(t, s.member, project, map[string]any{"title": "B", "start_date": "2026-03-05"})
	c := s.item(t, s.member, project, map[string]any{"title": "C", "parent_id": a.ID})
	other := s.project(t, s.lead, map[string]any{"identifier": "OTR", "name": "Otro"}).ID
	x := s.item(t, s.member, other, map[string]any{"title": "X"})

	dep := func(c *Client, from, to workItemDTO, typ string, lag int) *Response {
		return c.Post("/api/work-items/"+from.ID.String()+"/dependencies", map[string]any{"to_id": to.ID, "type": typ, "lag_days": lag})
	}
	dep(s.member, a, b, "blocks", 0).Expect(t, http.StatusCreated)
	dep(s.member, a, b, "blocks", 0).Expect(t, http.StatusConflict, "dependency_conflict")
	dep(s.member, b, a, "blocks", 0).Expect(t, http.StatusConflict, "dependency_cycle")
	dep(s.member, a, a, "relates", 0).Expect(t, http.StatusBadRequest, "dependency_hierarchy")
	dep(s.member, a, c, "relates", 0).Expect(t, http.StatusBadRequest, "dependency_hierarchy")
	dep(s.member, c, a, "precedes", 0).Expect(t, http.StatusBadRequest, "dependency_hierarchy")
	dep(s.member, a, b, "relates", 3).Expect(t, http.StatusBadRequest, "validation_failed")
	dep(s.member, a, b, "precedes", 400).Expect(t, http.StatusBadRequest, "validation_failed")
	dep(s.member, a, x, "duplicates", 0).Expect(t, http.StatusCreated) // across projects (RF-90)
	dep(s.outsider, x, a, "relates", 0).Expect(t, http.StatusForbidden)

	var res struct {
		Items []struct {
			Type          string `json:"type"`
			DatesViolated bool   `json:"dates_violated"`
		} `json:"items"`
	}
	dep(s.member, a, b, "precedes", 0).Expect(t, http.StatusCreated).JSON(t, &res)
	violated := false
	for _, d := range res.Items {
		if d.Type == "precedes" {
			violated = d.DatesViolated
		}
	}
	if !violated {
		t.Fatalf("B starts before A ends: dates_violated should be set (RF-96): %+v", res.Items)
	}

	// B cannot close while A is open (RF-95).
	cerrado := s.env.CatalogID("statuses", "Cerrado")
	res2 := s.patch(s.member, s.get(t, b.ID), map[string]any{"status_id": cerrado}).Expect(t, http.StatusConflict, "blocked_by_open_item")
	if !strings.Contains(string(res2.Body), a.Key) {
		t.Fatalf("blocker not named: %s", res2.Body)
	}
	s.patch(s.member, s.get(t, a.ID), map[string]any{"status_id": cerrado}).Expect(t, http.StatusOK)
	s.patch(s.member, s.get(t, b.ID), map[string]any{"status_id": cerrado}).Expect(t, http.StatusOK)
}

func TestConcurrentBlocksCannotBuildACycle(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	a := s.item(t, s.member, project, map[string]any{"title": "A"})
	b := s.item(t, s.member, project, map[string]any{"title": "B"})
	var wg sync.WaitGroup
	statuses := make([]int, 2)
	for i, pair := range [][2]workItemDTO{{a, b}, {b, a}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses[i] = s.member.Post("/api/work-items/"+pair[0].ID.String()+"/dependencies",
				map[string]any{"to_id": pair[1].ID, "type": "blocks"}).Status
		}()
	}
	wg.Wait()
	if (statuses[0] == http.StatusCreated) == (statuses[1] == http.StatusCreated) {
		t.Fatalf("statuses = %v, want exactly one dependency accepted (RF-94)", statuses)
	}
}

func TestWorkItemListFilters(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	bug := s.env.CatalogID("work_item_types", "Bug")
	s.item(t, s.member, project, map[string]any{"title": "Instalación", "assignee_id": s.memberID})
	s.item(t, s.member, project, map[string]any{"title": "Error de login", "type_id": bug})
	closed := s.item(t, s.member, project, map[string]any{"title": "Hecho"})
	s.patch(s.member, closed, map[string]any{"status_id": s.env.CatalogID("statuses", "Cerrado")}).Expect(t, http.StatusOK)

	count := func(query string) int {
		t.Helper()
		var page struct {
			Total int `json:"total"`
		}
		s.viewer.Get("/api/projects/"+project.String()+"/work-items"+query).Expect(t, http.StatusOK).JSON(t, &page)
		return page.Total
	}
	cases := map[string]int{
		"":                                    3,
		"?q=instalacion":                      1,
		"?q=KAI-2":                            1,
		"?type_id=" + bug.String():            1,
		"?closed=true":                        1,
		"?closed=false":                       2,
		"?assignee_id=" + s.memberID.String(): 1,
		"?assignee_id=none":                   2,
		"?page_size=200":                      3,
	}
	for q, want := range cases {
		if got := count(q); got != want {
			t.Errorf("%q: total = %d, want %d (RF-129)", q, got, want)
		}
	}
	for _, bad := range []string{"?type_id=x", "?closed=maybe", "?page_size=201", "?milestone_id=1"} {
		s.viewer.Get("/api/projects/"+project.String()+"/work-items"+bad).Expect(t, http.StatusBadRequest, "invalid_filter")
	}
}
