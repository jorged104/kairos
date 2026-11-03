package apitest

import (
	"net/http"
	"net/url"
	"slices"
	"testing"

	"github.com/google/uuid"
)

type planNode struct {
	ID           uuid.UUID   `json:"id"`
	Key          string      `json:"key"`
	StartDate    *string     `json:"start_date"`
	Blocked      bool        `json:"blocked"`
	MilestoneIDs []uuid.UUID `json:"milestone_ids"`
	ClosedAt     *string     `json:"closed_at"`
	Successors   []struct {
		ToID uuid.UUID `json:"to_id"`
		Type string    `json:"type"`
	} `json:"successors"`
}

func (s *workSetup) filtered(t *testing.T, c *Client, project uuid.UUID, q url.Values) []string {
	t.Helper()
	var out struct {
		Items []planNode `json:"items"`
	}
	c.Get("/api/projects/"+project.String()+"/tree?"+q.Encode()).Expect(t, http.StatusOK).JSON(t, &out)
	keys := []string{}
	for _, n := range out.Items {
		keys = append(keys, n.Key)
	}
	return keys
}

func TestFilters(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	enCurso, cerrado := s.env.CatalogID("statuses", "En progreso"), s.env.CatalogID("statuses", "Cerrado")
	alta := s.env.CatalogID("priorities", "Alta")
	m := s.env.CatalogID("work_item_sizes", "M")
	front := s.label(t, "Frontend")

	// KAI-1 to KAI-5, each with something different.
	a := s.item(t, s.member, project, map[string]any{"title": "Login", "assignee_ids": []uuid.UUID{s.memberID}, "label_ids": []uuid.UUID{front},
		"due_date": "2026-03-01", "start_date": "2026-02-20"})
	b := s.item(t, s.member, project, map[string]any{"title": "Informe", "priority_id": alta, "size_id": m, "due_date": "2026-04-01"})
	c := s.item(t, s.member, project, map[string]any{"title": "Base de datos", "assignee_ids": []uuid.UUID{s.leadID}})
	d := s.item(t, s.member, project, map[string]any{"title": "Cerrada"})
	s.item(t, s.member, project, map[string]any{"title": "Sin nada"})
	s.patch(s.admin, s.get(t, b.ID), map[string]any{"status_id": enCurso, "priority_id": alta, "size_id": m, "due_date": "2026-04-01"}).Expect(t, http.StatusOK)
	s.patch(s.admin, s.get(t, d.ID), map[string]any{"status_id": cerrado}).Expect(t, http.StatusOK)
	s.member.Post("/api/work-items/"+b.ID.String()+"/dependencies", map[string]any{"to_id": c.ID, "type": "blocks"}).Expect(t, http.StatusCreated)
	ms := s.milestone(t, project, map[string]any{"name": "Piloto", "due_date": "2026-05-01"})
	s.link(t, s.lead, ms.ID, a.ID).Expect(t, http.StatusNoContent)

	cases := []struct {
		q    url.Values
		want []string
	}{
		{url.Values{"status_id": {enCurso.String()}}, []string{"KAI-2"}},
		{url.Values{"status_id": {enCurso.String(), cerrado.String()}}, []string{"KAI-2", "KAI-4"}}, // or (RF-20)
		{url.Values{"priority_id": {alta.String()}, "status_id": {cerrado.String()}}, []string{}},   // and
		{url.Values{"size_id": {m.String()}}, []string{"KAI-2"}},
		{url.Values{"size_id": {"none"}, "closed": {"false"}}, []string{"KAI-1", "KAI-3", "KAI-5"}},
		{url.Values{"assignee_id": {"me"}}, []string{"KAI-1"}}, // the member looks (RF-19)
		{url.Values{"assignee_id": {"none"}}, []string{"KAI-2", "KAI-4", "KAI-5"}},
		{url.Values{"assignee_id": {"none", s.leadID.String()}}, []string{"KAI-2", "KAI-3", "KAI-4", "KAI-5"}},
		{url.Values{"label_id": {front.String()}}, []string{"KAI-1"}},
		{url.Values{"milestone_id": {ms.ID.String()}}, []string{"KAI-1"}},
		{url.Values{"blocked": {"true"}}, []string{"KAI-3"}},
		{url.Values{"due_before": {"2026-03-15"}}, []string{"KAI-1"}},
		{url.Values{"due_after": {"2026-03-15"}}, []string{"KAI-2"}},
		{url.Values{"overdue": {"true"}, "today": {"2026-03-20"}}, []string{"KAI-1"}},
		{url.Values{"q": {"inform"}}, []string{"KAI-2"}},
		{url.Values{"q": {"KAI-3"}}, []string{"KAI-3"}},
	}
	for _, cs := range cases {
		got := s.filtered(t, s.member, project, cs.q)
		slices.Sort(got)
		if !slices.Equal(got, cs.want) {
			t.Errorf("%v = %v, want %v", cs.q, got, cs.want)
		}
	}

	// The list sorts (RF-57).
	var page struct {
		Items []planNode `json:"items"`
	}
	s.member.Get("/api/projects/"+project.String()+"/work-items?sort=-due&closed=false").Expect(t, http.StatusOK).JSON(t, &page)
	if len(page.Items) < 2 || page.Items[0].Key != "KAI-2" || page.Items[1].Key != "KAI-1" {
		t.Fatalf("sorted by due desc = %+v", page.Items)
	}
	s.member.Get("/api/projects/"+project.String()+"/work-items?sort=bogus").Expect(t, http.StatusBadRequest, "invalid_filter")
	s.member.Get("/api/projects/"+project.String()+"/tree?due_before=ayer").Expect(t, http.StatusBadRequest, "invalid_filter")

	// The tree brings what the kanban and the timeline need (RF-61, RF-70).
	var tree struct {
		Items []planNode `json:"items"`
	}
	s.member.Get("/api/projects/"+project.String()+"/tree").Expect(t, http.StatusOK).JSON(t, &tree)
	for _, n := range tree.Items {
		switch n.ID {
		case a.ID:
			if n.StartDate == nil || *n.StartDate != "2026-02-20" || !slices.Equal(n.MilestoneIDs, []uuid.UUID{ms.ID}) {
				t.Fatalf("KAI-1 = %+v", n)
			}
		case b.ID:
			if len(n.Successors) != 1 || n.Successors[0].ToID != c.ID || n.Successors[0].Type != "blocks" {
				t.Fatalf("KAI-2 = %+v, want it blocking KAI-3 (RF-79)", n)
			}
		case c.ID:
			if !n.Blocked {
				t.Fatalf("KAI-3 = %+v, want blocked", n)
			}
		case d.ID:
			if n.ClosedAt == nil {
				t.Fatalf("KAI-4 = %+v, want its closing time (RF-68)", n)
			}
		}
	}
}
