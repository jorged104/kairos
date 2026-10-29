package db_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// The page version only moves when the description changes, whoever saves it (RF-34, RF-59).
func TestPageVersionFollowsTheDescription(t *testing.T) {
	f := newWorkFixture(t)
	ctx := context.Background()
	wi := f.workItem(t, f.projectA, 1, nil, 0)
	version := func() (page int) {
		t.Helper()
		f.mustScan(t, "SELECT page_version FROM work_items WHERE id = $1", &page, wi)
		return page
	}
	exec := func(sql string) {
		t.Helper()
		if _, err := f.pool.Exec(ctx, sql, wi); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	if v := version(); v != 1 {
		t.Fatalf("initial page_version = %d", v)
	}
	exec("UPDATE work_items SET title = 'Otro', done_ratio = 30 WHERE id = $1")
	if v := version(); v != 1 {
		t.Fatalf("page_version after a property change = %d, want 1", v)
	}
	exec(`UPDATE work_items SET description = '{"type":"doc","content":[]}' WHERE id = $1`)
	exec(`UPDATE work_items SET description = '{"type":"doc","content":[]}' WHERE id = $1`)
	if v := version(); v != 2 {
		t.Fatalf("page_version after a description change = %d, want 2", v)
	}
	exec("UPDATE work_items SET description = NULL WHERE id = $1")
	if v := version(); v != 3 {
		t.Fatalf("page_version after clearing = %d, want 3", v)
	}

	var m uuid.UUID
	var milestone, project int
	f.mustScan(t, "INSERT INTO milestones (project_id, name, due_date) VALUES ($1, 'H', '2026-12-01') RETURNING id", &m, f.projectA)
	f.mustScan(t, `UPDATE milestones SET description = '{"type":"doc"}' WHERE id = $1 RETURNING page_version`, &milestone, m)
	f.mustScan(t, `UPDATE projects SET description = '{"type":"doc"}' WHERE id = $1 RETURNING page_version`, &project, f.projectA)
	if milestone != 2 || project != 2 {
		t.Fatalf("milestone %d, project %d, want 2 and 2", milestone, project)
	}
}
