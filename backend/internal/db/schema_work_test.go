package db_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"openscum/internal/db"
	"openscum/internal/db/dbtest"
)

// workFixture holds the rows the spec 004 schema tests build on.
type workFixture struct {
	pool               *pgxpool.Pool
	user, team         uuid.UUID
	status, closed     uuid.UUID
	typ, priority      uuid.UUID
	projectA, projectB uuid.UUID
}

func newWorkFixture(t *testing.T) *workFixture {
	t.Helper()
	store := db.NewStore(dbtest.New(t))
	f := &workFixture{pool: store.Pool, user: createUser(t, store, "owner@example.com").ID}
	f.mustScan(t, "INSERT INTO teams (name, name_normalized) VALUES ('BI', 'bi') RETURNING id", &f.team)
	f.mustScan(t, "SELECT id FROM statuses WHERE is_default", &f.status)
	f.mustScan(t, "SELECT id FROM statuses WHERE name = 'Cerrado'", &f.closed)
	f.mustScan(t, "SELECT id FROM work_item_types WHERE name = 'Actividad'", &f.typ)
	f.mustScan(t, "SELECT id FROM priorities WHERE is_default", &f.priority)
	f.projectA = f.project(t, "AAA")
	f.projectB = f.project(t, "BBB")
	return f
}

func (f *workFixture) mustScan(t *testing.T, sql string, dst any, args ...any) {
	t.Helper()
	if err := f.pool.QueryRow(context.Background(), sql, args...).Scan(dst); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func (f *workFixture) project(t *testing.T, identifier string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	f.mustScan(t, `INSERT INTO projects (team_id, owner_id, identifier, name) VALUES ($1, $2, $3, $3) RETURNING id`,
		&id, f.team, f.user, identifier)
	return id
}

func (f *workFixture) workItem(t *testing.T, project uuid.UUID, seq int, parent *uuid.UUID, depth int) uuid.UUID {
	t.Helper()
	id, err := f.insertWorkItem(project, seq, parent, depth)
	if err != nil {
		t.Fatalf("insert work item: %v", err)
	}
	return id
}

func (f *workFixture) insertWorkItem(project uuid.UUID, seq int, parent *uuid.UUID, depth int) (uuid.UUID, error) {
	var id uuid.UUID
	err := f.pool.QueryRow(context.Background(), `
		INSERT INTO work_items (project_id, type_id, status_id, priority_id, parent_id, sequence_number, title, author_id, depth)
		VALUES ($1, $2, $3, $4, $5, $6, 'item', $7, $8) RETURNING id`,
		project, f.typ, f.status, f.priority, parent, seq, f.user, depth).Scan(&id)
	return id, err
}

func (f *workFixture) mustFail(t *testing.T, why, sql string, args ...any) {
	t.Helper()
	if _, err := f.pool.Exec(context.Background(), sql, args...); err == nil {
		t.Fatalf("%s: the database accepted it", why)
	}
}

func TestWorkSeedData(t *testing.T) {
	f := newWorkFixture(t)
	var n int
	f.mustScan(t, "SELECT count(*) FROM statuses WHERE is_default", &n)
	if n != 1 {
		t.Fatalf("default statuses = %d, want 1", n)
	}
	f.mustScan(t, "SELECT count(*) FROM priorities WHERE is_default", &n)
	if n != 1 {
		t.Fatalf("default priorities = %d, want 1", n)
	}
	f.mustScan(t, "SELECT count(*) FROM phase_definitions p JOIN methodologies m ON m.id = p.methodology_id WHERE m.name = 'PRINCE2'", &n)
	if n != 4 {
		t.Fatalf("PRINCE2 phases = %d, want 4", n)
	}
	f.mustFail(t, "second default status", "INSERT INTO statuses (name, name_normalized, is_default) VALUES ('Otro', 'otro', true)")
	f.mustFail(t, "closed default status", "UPDATE statuses SET is_closed = true WHERE is_default")
	f.mustFail(t, "second default priority", "INSERT INTO priorities (name, name_normalized, is_default) VALUES ('Otra', 'otra', true)")
	f.mustFail(t, "duplicate active type", "INSERT INTO work_item_types (name, name_normalized) VALUES ('BUG', 'bug')")
}

func TestWorkSameProjectIsEnforcedByTheDatabase(t *testing.T) {
	f := newWorkFixture(t)
	ctx := context.Background()
	parent := f.workItem(t, f.projectA, 1, nil, 0)

	if _, err := f.insertWorkItem(f.projectB, 1, &parent, 1); err == nil {
		t.Fatal("child in another project than its parent was accepted (RF-76)")
	}

	var milestoneB, phaseA uuid.UUID
	f.mustScan(t, "INSERT INTO milestones (project_id, name) VALUES ($1, 'M1') RETURNING id", &milestoneB, f.projectB)
	f.mustFail(t, "milestone of another project (RF-51)",
		"INSERT INTO milestone_work_items (milestone_id, work_item_id, project_id) VALUES ($1, $2, $3)", milestoneB, parent, f.projectB)
	f.mustFail(t, "milestone of another project (RF-51)",
		"INSERT INTO milestone_work_items (milestone_id, work_item_id, project_id) VALUES ($1, $2, $3)", milestoneB, parent, f.projectA)

	f.mustScan(t, `INSERT INTO project_phases (project_id, definition_id)
		SELECT $1, id FROM phase_definitions LIMIT 1 RETURNING id`, &phaseA, f.projectA)
	f.mustFail(t, "phase of another project (RF-58)", "UPDATE milestones SET phase_id = $1 WHERE id = $2", phaseA, milestoneB)

	var milestoneA uuid.UUID
	f.mustScan(t, "INSERT INTO milestones (project_id, name, phase_id) VALUES ($1, 'M2', $2) RETURNING id", &milestoneA, f.projectA, phaseA)
	if _, err := f.pool.Exec(ctx, "INSERT INTO milestone_work_items (milestone_id, work_item_id, project_id) VALUES ($1, $2, $3)",
		milestoneA, parent, f.projectA); err != nil {
		t.Fatalf("same-project link rejected: %v", err)
	}
}

func TestWorkChecks(t *testing.T) {
	f := newWorkFixture(t)
	f.mustFail(t, "lowercase identifier (RF-19)",
		"INSERT INTO projects (team_id, owner_id, identifier, name) VALUES ($1, $2, 'kai', 'x')", f.team, f.user)
	f.mustFail(t, "identifier starting with a digit (RF-19)",
		"INSERT INTO projects (team_id, owner_id, identifier, name) VALUES ($1, $2, '1KAI', 'x')", f.team, f.user)
	f.mustFail(t, "duplicate identifier (RF-20)",
		"INSERT INTO projects (team_id, owner_id, identifier, name) VALUES ($1, $2, 'AAA', 'x')", f.team, f.user)
	f.mustFail(t, "end before start (RF-24)",
		"UPDATE projects SET start_date = '2026-02-01', end_date = '2026-01-01' WHERE id = $1", f.projectA)

	wi := f.workItem(t, f.projectA, 1, nil, 0)
	f.mustFail(t, "due before start (RF-66)", "UPDATE work_items SET start_date = '2026-02-01', due_date = '2026-01-01' WHERE id = $1", wi)
	f.mustFail(t, "estimate over 10000 (RF-67)", "UPDATE work_items SET estimated_hours = 10001 WHERE id = $1", wi)
	f.mustFail(t, "negative remaining (RF-67)", "UPDATE work_items SET remaining_hours = -1 WHERE id = $1", wi)
	f.mustFail(t, "ratio over 100 (RF-67)", "UPDATE work_items SET done_ratio = 101 WHERE id = $1", wi)
	f.mustFail(t, "empty title (RF-72)", "UPDATE work_items SET title = '  ' WHERE id = $1", wi)
	f.mustFail(t, "depth 10 (RF-78)", "UPDATE work_items SET depth = 10 WHERE id = $1", wi)
	f.mustFail(t, "weight over 100 (RF-152)", "UPDATE work_items SET weight = 101 WHERE id = $1", wi)
	f.mustFail(t, "negative weight (RF-152)", "UPDATE work_items SET weight = -1 WHERE id = $1", wi)
	f.mustFail(t, "type default weight over 100 (RF-152)", "UPDATE work_item_types SET default_weight = 101 WHERE id = $1", f.typ)
	f.mustFail(t, "duplicate sequence (RF-63)",
		`INSERT INTO work_items (project_id, type_id, status_id, priority_id, sequence_number, title, author_id)
		 VALUES ($1, $2, $3, $4, 1, 'x', $5)`, f.projectA, f.typ, f.status, f.priority, f.user)

	f.mustFail(t, "fulfilled milestone without date (RF-46)",
		"INSERT INTO milestones (project_id, name, status) VALUES ($1, 'M', 'cumplido')", f.projectA)
	f.mustFail(t, "open milestone with date (RF-47)",
		"INSERT INTO milestones (project_id, name, completed_on) VALUES ($1, 'M', '2026-01-01')", f.projectA)

	other := f.workItem(t, f.projectA, 2, nil, 0)
	f.mustFail(t, "self dependency (RF-92)",
		"INSERT INTO work_item_dependencies (from_id, to_id, type, created_by) VALUES ($1, $1, 'blocks', $2)", wi, f.user)
	f.mustFail(t, "lag on blocks (RF-91)",
		"INSERT INTO work_item_dependencies (from_id, to_id, type, lag_days, created_by) VALUES ($1, $2, 'blocks', 2, $3)", wi, other, f.user)
	f.mustFail(t, "lag over a year (RF-91)",
		"INSERT INTO work_item_dependencies (from_id, to_id, type, lag_days, created_by) VALUES ($1, $2, 'precedes', 366, $3)", wi, other, f.user)

	f.mustFail(t, "team scope without team (RF-98)",
		"INSERT INTO objectives (title, owner_id, scope, period_start, period_end) VALUES ('O', $1, 'equipo', '2026-01-01', '2026-03-31')", f.user)
	var obj uuid.UUID
	f.mustScan(t, "INSERT INTO objectives (title, owner_id, scope, period_start, period_end) VALUES ('O', $1, 'organizacion', '2026-01-01', '2026-03-31') RETURNING id", &obj, f.user)
	f.mustFail(t, "target equal to start (RF-102)",
		"INSERT INTO key_results (objective_id, title, start_value, target_value, current_value, calc_mode) VALUES ($1, 'KR', 5, 5, 5, 'manual')", obj)
}

func TestKeyResultMilestonesFollowTheObjective(t *testing.T) {
	f := newWorkFixture(t)
	ctx := context.Background()
	var obj, kr, m uuid.UUID
	f.mustScan(t, "INSERT INTO objectives (title, owner_id, scope, period_start, period_end) VALUES ('O', $1, 'organizacion', '2026-01-01', '2026-03-31') RETURNING id", &obj, f.user)
	f.mustScan(t, "INSERT INTO key_results (objective_id, title, start_value, target_value, current_value, calc_mode) VALUES ($1, 'KR', 0, 10, 0, 'derivado') RETURNING id", &kr, obj)
	f.mustScan(t, "INSERT INTO milestones (project_id, name) VALUES ($1, 'M') RETURNING id", &m, f.projectA)

	f.mustFail(t, "milestone not linked to the objective (RF-108)",
		"INSERT INTO key_result_milestones (key_result_id, objective_id, milestone_id) VALUES ($1, $2, $3)", kr, obj, m)
	if _, err := f.pool.Exec(ctx, "INSERT INTO objective_milestones (objective_id, milestone_id) VALUES ($1, $2)", obj, m); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, "INSERT INTO key_result_milestones (key_result_id, objective_id, milestone_id) VALUES ($1, $2, $3)", kr, obj, m); err != nil {
		t.Fatalf("link after objective link: %v", err)
	}
	if _, err := f.pool.Exec(ctx, "DELETE FROM objective_milestones WHERE objective_id = $1 AND milestone_id = $2", obj, m); err != nil {
		t.Fatal(err)
	}
	var n int
	f.mustScan(t, "SELECT count(*) FROM key_result_milestones", &n)
	if n != 0 {
		t.Fatalf("key result milestones after unlinking = %d, want 0 (RF-109)", n)
	}
}

func TestWorkItemAggregates(t *testing.T) {
	f := newWorkFixture(t)
	ctx := context.Background()
	root := f.workItem(t, f.projectA, 1, nil, 0)
	a := f.workItem(t, f.projectA, 2, &root, 1)
	b := f.workItem(t, f.projectA, 3, &root, 1)
	leaf := f.workItem(t, f.projectA, 4, &a, 2)

	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := f.pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	// a: 10 h at 50 %, leaf: 30 h closed, b: no estimate at 20 %.
	exec("UPDATE work_items SET estimated_hours = 10, remaining_hours = 5, done_ratio = 50 WHERE id = $1", a)
	exec("UPDATE work_items SET estimated_hours = 30, remaining_hours = 0, status_id = $2 WHERE id = $1", leaf, f.closed)
	exec("UPDATE work_items SET done_ratio = 20 WHERE id = $1", b)
	for _, id := range []uuid.UUID{leaf, b} {
		exec("SELECT work_item_refresh_up($1)", id)
	}

	check := func(id uuid.UUID, est, rem float64, ratio int) {
		t.Helper()
		var gotEst, gotRem float64
		var gotRatio int
		if err := f.pool.QueryRow(ctx, "SELECT agg_estimated::float8, agg_remaining::float8, derived_done_ratio FROM work_items WHERE id = $1",
			id).Scan(&gotEst, &gotRem, &gotRatio); err != nil {
			t.Fatal(err)
		}
		if gotEst != est || gotRem != rem || gotRatio != ratio {
			t.Fatalf("aggregates = %v h, %v h, %d %%; want %v h, %v h, %d %%", gotEst, gotRem, gotRatio, est, rem, ratio)
		}
	}
	// a: (10*50 + 30*100) / 40 = 87.5 → 88 (RF-80, RF-81).
	check(a, 40, 5, 88)
	// root: same weighted sum; b and root weigh 0 with no estimate.
	check(root, 40, 5, 88)
	// b alone, no estimates anywhere: simple mean (RF-82).
	check(b, 0, 0, 20)

	// A full rebuild gives the same values.
	exec("UPDATE work_items SET agg_estimated = 0, agg_remaining = 0, agg_count = 1, agg_ratio_sum = 0, agg_est_ratio_sum = 0")
	exec("SELECT work_item_recompute_project($1)", f.projectA)
	check(root, 40, 5, 88)
	check(b, 0, 0, 20)
}

func TestWorkItemWeightedAggregates(t *testing.T) {
	f := newWorkFixture(t)
	ctx := context.Background()
	root := f.workItem(t, f.projectA, 1, nil, 0)
	a := f.workItem(t, f.projectA, 2, &root, 1)
	b := f.workItem(t, f.projectA, 3, &root, 1)
	leaf := f.workItem(t, f.projectA, 4, &a, 2)

	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := f.pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	// a: weight 3 at 50 %, leaf: weight 1 closed, b: tracking (weight 0) at 20 % with 8 h.
	exec("UPDATE work_items SET weight = 3, done_ratio = 50 WHERE id = $1", a)
	exec("UPDATE work_items SET status_id = $2 WHERE id = $1", leaf, f.closed)
	exec("UPDATE work_items SET weight = 0, done_ratio = 20, estimated_hours = 8 WHERE id = $1", b)
	for _, id := range []uuid.UUID{leaf, b} {
		exec("SELECT work_item_refresh_up($1)", id)
	}

	check := func(id uuid.UUID, weight int64, ratio, legacy int, est float64) {
		t.Helper()
		var gotWeight int64
		var gotRatio, gotLegacy int
		var gotEst float64
		if err := f.pool.QueryRow(ctx,
			"SELECT agg_weight, weighted_done_ratio, derived_done_ratio, agg_estimated::float8 FROM work_items WHERE id = $1",
			id).Scan(&gotWeight, &gotRatio, &gotLegacy, &gotEst); err != nil {
			t.Fatal(err)
		}
		if gotWeight != weight || gotRatio != ratio || gotLegacy != legacy || gotEst != est {
			t.Fatalf("weight %d, ratio %d %%, legacy %d %%, %v h; want %d, %d %%, %d %%, %v h",
				gotWeight, gotRatio, gotLegacy, gotEst, weight, ratio, legacy, est)
		}
	}
	// a: (3*50 + 1*100) / 4 = 62.5 → 63 (RF-153).
	check(a, 4, 63, 75, 0)
	// root: (1*0 + 250) / 5 = 50; b weighs 0 and doesn't count, but its hours do (RF-159).
	// The previous formula is still kept up to date for the previous API: b has the only
	// estimate, so it is b's 20 %.
	check(root, 5, 50, 20, 8)
	// b alone weighs 0: simple mean of its own ratio (RF-153).
	check(b, 0, 20, 20, 8)

	// A full rebuild gives the same values.
	exec("UPDATE work_items SET agg_weight = 1, agg_weight_ratio_sum = 0")
	exec("SELECT work_item_recompute_project($1)", f.projectA)
	check(root, 5, 50, 20, 8)
	check(a, 4, 63, 75, 0)
}
