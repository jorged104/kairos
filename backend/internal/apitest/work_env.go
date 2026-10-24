package apitest

import (
	"context"

	"github.com/google/uuid"
)

// CatalogID returns the id of an active catalog row by name, e.g.
// CatalogID("statuses", "Nuevo"). table comes from the tests, never from input.
func (e *Env) CatalogID(table, name string) uuid.UUID {
	e.t.Helper()
	var id uuid.UUID
	if err := e.Store.Pool.QueryRow(context.Background(),
		"SELECT id FROM "+table+" WHERE name = $1 AND retired_at IS NULL", name).Scan(&id); err != nil {
		e.t.Fatalf("catalog %s %q: %v", table, name, err)
	}
	return id
}

// Admin seeds an admin and returns a client signed in as them.
func (e *Env) Admin() *Client {
	e.t.Helper()
	return e.As(e.CreateUser(UserSpec{Email: "admin-" + uuid.NewString()[:8] + "@x.com", Roles: []string{"admin"}}))
}

// SeedProject inserts a project straight into the database.
func (e *Env) SeedProject(identifier string, team, owner uuid.UUID) uuid.UUID {
	e.t.Helper()
	var id uuid.UUID
	if err := e.Store.Pool.QueryRow(context.Background(),
		"INSERT INTO projects (team_id, owner_id, identifier, name) VALUES ($1, $2, $3, $3) RETURNING id",
		team, owner, identifier).Scan(&id); err != nil {
		e.t.Fatalf("seed project: %v", err)
	}
	return id
}

// SeedWorkItem inserts a root work item with the given status straight into the database.
func (e *Env) SeedWorkItem(project, status, author uuid.UUID) uuid.UUID {
	e.t.Helper()
	var id uuid.UUID
	if err := e.Store.Pool.QueryRow(context.Background(), `
		WITH n AS (UPDATE projects SET wi_counter = wi_counter + 1 WHERE id = $1 RETURNING wi_counter)
		INSERT INTO work_items (project_id, type_id, status_id, priority_id, sequence_number, title, author_id)
		SELECT $1, (SELECT id FROM work_item_types WHERE name = 'Actividad'), $2,
		       (SELECT id FROM priorities WHERE is_default), n.wi_counter, 'seed', $3 FROM n
		RETURNING id`, project, status, author).Scan(&id); err != nil {
		e.t.Fatalf("seed work item: %v", err)
	}
	return id
}
