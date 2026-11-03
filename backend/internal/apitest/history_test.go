package apitest

import (
	"net/http"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
)

type historyDTO struct {
	Entries []struct {
		Actor struct {
			Name string `json:"name"`
		} `json:"actor"`
		Kind    string `json:"kind"`
		Changes []struct {
			Field string `json:"field"`
			From  any    `json:"from"`
			To    any    `json:"to"`
		} `json:"changes"`
	} `json:"entries"`
	Next *int64 `json:"next"`
}

func (s *workSetup) history(t *testing.T, c *Client, id uuid.UUID, before *int64) historyDTO {
	t.Helper()
	url := "/api/work-items/" + id.String() + "/history"
	if before != nil {
		url += "?before=" + strconv.FormatInt(*before, 10)
	}
	var h historyDTO
	c.Get(url).Expect(t, http.StatusOK).JSON(t, &h)
	return h
}

func name(v any) string {
	m, _ := v.(map[string]any)
	n, _ := m["name"].(string)
	return n
}

func TestHistory(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	front := s.label(t, "Frontend")
	wi := s.item(t, s.member, project, map[string]any{"title": "Login"})
	other := s.item(t, s.member, project, map[string]any{"title": "Base"})

	s.patch(s.admin, s.get(t, wi.ID), map[string]any{"status_id": s.env.CatalogID("statuses", "En progreso"),
		"assignee_ids": []uuid.UUID{s.memberID}, "label_ids": []uuid.UUID{front}}).Expect(t, http.StatusOK)
	owner := "work-items/" + wi.ID.String()
	saved := s.savePage(t, s.member, owner, 1, paragraph("uno"), http.StatusOK)
	saved = s.savePage(t, s.member, owner, saved.PageVersion, paragraph("dos"), http.StatusOK)
	s.savePage(t, s.lead, owner, saved.PageVersion, paragraph("tres"), http.StatusOK)
	s.member.Post("/api/work-items/"+wi.ID.String()+"/dependencies", map[string]any{"to_id": other.ID, "type": "relates"}).Expect(t, http.StatusCreated)

	// Newest first, with readable names (RF-48, RF-49, RF-52); the viewer reads it too (RF-53).
	h := s.history(t, s.viewer, wi.ID, nil)
	kinds := []string{}
	for _, e := range h.Entries {
		kinds = append(kinds, e.Kind)
	}
	// The member's two page saves are one entry; the lead's is another (RF-51).
	want := []string{"dependency_added", "description", "description", "updated", "created"}
	if len(kinds) != len(want) {
		t.Fatalf("kinds = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("kinds = %v, want %v", kinds, want)
		}
	}
	if h.Next != nil {
		t.Fatalf("next = %v, want none", *h.Next)
	}
	dep := h.Entries[0].Changes[0]
	if dep.Field != "relates" || name(dep.To) != "KAI-2 Base" {
		t.Fatalf("dependency = %+v", dep)
	}
	if desc := h.Entries[2]; desc.Actor.Name != "Mario" || len(desc.Changes) != 1 || desc.Changes[0].To != nil {
		t.Fatalf("description = %+v, want only «editó la descripción» (RF-50)", desc)
	}
	got := map[string][2]string{}
	for _, c := range h.Entries[3].Changes {
		switch c.Field {
		case "status_id":
			got[c.Field] = [2]string{name(c.From), name(c.To)}
		case "assignee_ids", "label_ids":
			to, _ := c.To.([]any)
			if len(to) == 1 {
				got[c.Field] = [2]string{"", name(to[0])}
			}
		}
	}
	if got["status_id"] != [2]string{"Nuevo", "En progreso"} || got["assignee_ids"][1] != "Mario" || got["label_ids"][1] != "Frontend" {
		t.Fatalf("changes = %+v", h.Entries[3].Changes)
	}

	// 50 entries per page, then the rest (RF-52).
	for i := range 55 {
		s.patch(s.member, s.get(t, other.ID), map[string]any{"title": "Base " + strconv.Itoa(i)}).Expect(t, http.StatusOK)
	}
	first := s.history(t, s.member, other.ID, nil)
	if len(first.Entries) != 50 || first.Next == nil {
		t.Fatalf("first page = %d entries, next %v", len(first.Entries), first.Next)
	}
	second := s.history(t, s.member, other.ID, first.Next)
	if len(second.Entries) != 6 || second.Next != nil || second.Entries[5].Kind != "created" {
		t.Fatalf("second page = %d entries, next %v", len(second.Entries), second.Next)
	}
	s.member.Get("/api/work-items/"+other.ID.String()+"/history?before=x").Expect(t, http.StatusBadRequest, "invalid_filter")
}

func TestMetricsAndHistoryPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("prueba de rendimiento omitida con -short")
	}
	s := newWorkSetup(t)
	project := s.workProject(t)
	enCurso, nuevo := s.env.CatalogID("statuses", "En progreso"), s.env.CatalogID("statuses", "Nuevo")
	wi := s.item(t, s.member, project, map[string]any{})
	for range 10 {
		blocker := s.item(t, s.member, project, map[string]any{})
		s.member.Post("/api/work-items/"+blocker.ID.String()+"/dependencies", map[string]any{"to_id": wi.ID, "type": "blocks"}).Expect(t, http.StatusCreated)
	}
	for i := range 200 {
		status := enCurso
		if i%2 == 1 {
			status = nuevo
		}
		s.patch(s.admin, s.get(t, wi.ID), map[string]any{"status_id": status}).Expect(t, http.StatusOK)
	}
	for _, c := range []struct {
		what string
		run  func()
	}{
		{"métricas", func() { s.metrics(t, s.member, wi.ID, "2026-03-06") }},
		{"historial", func() { s.history(t, s.member, wi.ID, nil) }},
	} {
		times := make([]time.Duration, 0, 5)
		for range 5 {
			start := time.Now()
			c.run()
			times = append(times, time.Since(start))
		}
		sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
		t.Logf("%s con 200 cambios: mediana %v (límite 300ms)", c.what, times[2].Round(time.Millisecond))
		if times[2] > 300*time.Millisecond {
			t.Errorf("%s: mediana %v supera 300ms", c.what, times[2])
		}
	}
}
