package apitest

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// TestWorkPerformance loads the design size of spec 004 and checks the
// response time limits of its non-functional requirements (median of 5).
func TestWorkPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("prueba de rendimiento omitida con -short")
	}
	s := newWorkSetup(t)
	ctx := context.Background()
	pool := s.env.Store.Pool
	act := s.env.CatalogID("work_item_types", "Actividad")
	nuevo := s.env.CatalogID("statuses", "Nuevo")
	cerrado := s.env.CatalogID("statuses", "Cerrado")
	normal := s.env.CatalogID("priorities", "Normal")
	start := time.Now()

	// 500 projects: KAI (5 000 work items) and 499 with 90 each = 49 910 in total.
	kai := s.project(t, s.lead, map[string]any{"identifier": "KAI", "name": "Kairos"}).ID
	projectRows := make([][]any, 0, 499)
	projectIDs := make([]uuid.UUID, 0, 499)
	for i := 0; i < 499; i++ {
		id := uuid.New()
		projectIDs = append(projectIDs, id)
		team := s.bi
		if i%2 == 0 {
			team = s.other
		}
		projectRows = append(projectRows, []any{id, team, s.leadID, fmt.Sprintf("P%04d", i), fmt.Sprintf("Proyecto %d", i), int32(90)})
	}
	if _, err := pool.CopyFrom(ctx, pgx.Identifier{"projects"}, []string{"id", "team_id", "owner_id", "identifier", "name", "wi_counter"},
		pgx.CopyFromRows(projectRows)); err != nil {
		t.Fatal(err)
	}

	cols := []string{"id", "project_id", "type_id", "status_id", "priority_id", "parent_id", "sequence_number", "title",
		"author_id", "estimated_hours", "remaining_hours", "depth"}
	var rows [][]any
	// KAI: 500 chains of 10 levels.
	kaiIDs := make([]uuid.UUID, 0, 5000)
	roots := make([]uuid.UUID, 0, 500)
	seq := int32(0)
	for c := 0; c < 500; c++ {
		var parent *uuid.UUID
		for d := 0; d < 10; d++ {
			id := uuid.New()
			seq++
			status := nuevo
			if seq%3 == 0 {
				status = cerrado
			}
			rows = append(rows, []any{id, kai, act, status, normal, parent, seq, fmt.Sprintf("Tarea %d", seq), s.leadID, 4.0, 2.0, int16(d)})
			kaiIDs = append(kaiIDs, id)
			if d == 0 {
				roots = append(roots, id)
			}
			p := id
			parent = &p
		}
	}
	for _, pid := range projectIDs {
		for n := int32(1); n <= 90; n++ {
			rows = append(rows, []any{uuid.New(), pid, act, nuevo, normal, (*uuid.UUID)(nil), n, "Tarea", s.leadID, 1.0, 1.0, int16(0)})
		}
	}
	if _, err := pool.CopyFrom(ctx, pgx.Identifier{"work_items"}, cols, pgx.CopyFromRows(rows)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE projects SET wi_counter = 5000 WHERE id = $1", kai); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "SELECT work_item_recompute_project($1)", kai); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO project_work_item_types (project_id, type_id) SELECT id, $1 FROM projects ON CONFLICT DO NOTHING", act); err != nil {
		t.Fatal(err)
	}

	// A milestone with 1 000 effective work items: 100 chains.
	m := s.milestone(t, kai, map[string]any{"name": "M1"})
	var links [][]any
	for _, r := range roots[:100] {
		links = append(links, []any{m.ID, r, kai})
	}
	if _, err := pool.CopyFrom(ctx, pgx.Identifier{"milestone_work_items"}, []string{"milestone_id", "work_item_id", "project_id"}, pgx.CopyFromRows(links)); err != nil {
		t.Fatal(err)
	}

	// 200 objectives with 10 derived key results, each linked to 5 work items and the milestone.
	var objRows, omRows, krRows, krmRows, krwRows [][]any
	var objective uuid.UUID
	for i := 0; i < 200; i++ {
		oid := uuid.New()
		objective = oid
		objRows = append(objRows, []any{oid, fmt.Sprintf("O%d", i), s.leadID, "equipo", s.bi, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC), "activo"})
		omRows = append(omRows, []any{oid, m.ID})
		for k := 0; k < 10; k++ {
			kr := uuid.New()
			krRows = append(krRows, []any{kr, oid, "KR", 0.0, 100.0, 0.0, "derivado"})
			krmRows = append(krmRows, []any{kr, oid, m.ID})
			for w := 0; w < 5; w++ {
				krwRows = append(krwRows, []any{kr, kaiIDs[(i*50+k*5+w)%len(kaiIDs)]})
			}
		}
	}
	for _, c := range []struct {
		table string
		cols  []string
		rows  [][]any
	}{
		{"objectives", []string{"id", "title", "owner_id", "scope", "scope_team_id", "period_start", "period_end", "status"}, objRows},
		{"objective_milestones", []string{"objective_id", "milestone_id"}, omRows},
		{"key_results", []string{"id", "objective_id", "title", "start_value", "target_value", "current_value", "calc_mode"}, krRows},
		{"key_result_milestones", []string{"key_result_id", "objective_id", "milestone_id"}, krmRows},
		{"key_result_work_items", []string{"key_result_id", "work_item_id"}, krwRows},
	} {
		if _, err := pool.CopyFrom(ctx, pgx.Identifier{c.table}, c.cols, pgx.CopyFromRows(c.rows)); err != nil {
			t.Fatalf("copy %s: %v", c.table, err)
		}
	}
	if _, err := pool.Exec(ctx, "ANALYZE"); err != nil {
		t.Fatal(err)
	}
	t.Logf("datos cargados en %v", time.Since(start).Round(time.Millisecond))

	// measure times only the call returned by prepare.
	measure := func(name string, limit time.Duration, prepare func() func() *Response) {
		t.Helper()
		var times []time.Duration
		for i := 0; i < 5; i++ {
			call := prepare()
			began := time.Now()
			res := call()
			times = append(times, time.Since(began))
			if res.Status >= 300 {
				t.Fatalf("%s: status %d %s", name, res.Status, res.Body)
			}
		}
		sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
		t.Logf("%-40s mediana %v (límite %v)", name, times[2].Round(time.Millisecond), limit)
		if times[2] > limit {
			t.Errorf("%s: mediana %v supera %v", name, times[2], limit)
		}
	}
	median := func(name string, limit time.Duration, fn func() *Response) {
		t.Helper()
		measure(name, limit, func() func() *Response { return fn })
	}
	c := s.lead
	median("listado de proyectos", 300*time.Millisecond, func() *Response { return c.Get("/api/projects?team_id=" + s.bi.String()) })
	median("listado de work items (200)", 300*time.Millisecond, func() *Response {
		return c.Get("/api/projects/" + kai.String() + "/work-items?page_size=200&closed=false&q=tarea")
	})
	median("work items por hito efectivo", 300*time.Millisecond, func() *Response {
		return c.Get("/api/projects/" + kai.String() + "/work-items?milestone_id=" + m.ID.String())
	})
	median("árbol de 5 000 work items", 500*time.Millisecond, func() *Response { return c.Get("/api/projects/" + kai.String() + "/tree") })
	median("ficha de hito con avance", 200*time.Millisecond, func() *Response { return c.Get("/api/milestones/" + m.ID.String()) })
	median("ficha de objetivo con avance", 300*time.Millisecond, func() *Response { return c.Get("/api/objectives/" + objective.String()) })
	median("listado de objetivos", 300*time.Millisecond, func() *Response { return c.Get("/api/objectives?page_size=100") })

	// Changes at the bottom of a 10-level chain refresh every ancestor.
	leaf := kaiIDs[9]
	measure("cambio de estado en profundidad 9", 200*time.Millisecond, func() func() *Response {
		wi := s.get(t, leaf)
		next := cerrado
		if wi.StatusID == cerrado {
			next = nuevo
		}
		return func() *Response { return s.patch(c, wi, map[string]any{"status_id": next}) }
	})
	measure("mover una cadena de 5 niveles", 200*time.Millisecond, func() func() *Response {
		wi := s.get(t, kaiIDs[15]) // depth 5 of chain 1, with 4 levels below
		target := roots[2+int(wi.Version)%50]
		return func() *Response { return s.patch(c, wi, map[string]any{"parent_id": target}) }
	})
	verifyAggregates(t, s.env, kai)
}
