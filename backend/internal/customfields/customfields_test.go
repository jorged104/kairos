package customfields

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/authz"
	"openscum/internal/db"
	"openscum/internal/db/dbtest"
	"openscum/internal/db/sqlc"
	"openscum/internal/fields"
)

type fixture struct {
	store             *db.Store
	svc               *Service
	admin             *authz.Principal
	project, workItem uuid.UUID
	bug, task         uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	store := db.NewStore(dbtest.New(t))
	f := &fixture{store: store, svc: NewService(store)}
	var user, team uuid.UUID
	scan := func(sql string, dst *uuid.UUID, args ...any) {
		t.Helper()
		if err := store.Pool.QueryRow(ctx, sql, args...).Scan(dst); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	scan("INSERT INTO users (name, email, email_normalized) VALUES ('A', 'a@x.com', 'a@x.com') RETURNING id", &user)
	scan("INSERT INTO teams (name, name_normalized) VALUES ('BI', 'bi') RETURNING id", &team)
	scan("INSERT INTO projects (team_id, owner_id, identifier, name) VALUES ($1, $2, 'KAI', 'Kairos') RETURNING id", &f.project, team, user)
	scan("SELECT id FROM work_item_types WHERE name = 'Bug'", &f.bug)
	scan("SELECT id FROM work_item_types WHERE name = 'Tarea'", &f.task)
	scan(`INSERT INTO work_items (project_id, type_id, status_id, priority_id, sequence_number, title, author_id)
		SELECT $1, $2, (SELECT id FROM statuses WHERE is_default), (SELECT id FROM priorities WHERE is_default), 1, 'x', $3
		RETURNING id`, &f.workItem, f.project, f.bug, user)
	f.admin = authz.NewPrincipal(user, "active", []authz.Role{authz.RoleAdmin}, nil)
	return f
}

func (f *fixture) field(t *testing.T, req fieldRequest) Field {
	t.Helper()
	out, err := f.svc.Create(context.Background(), f.admin, req)
	if err != nil {
		t.Fatalf("create field %+v: %v", req, err)
	}
	return out
}

// apply runs Apply for the fixture's project or work item in its own transaction.
func (f *fixture) apply(t *testing.T, entity string, enabled []uuid.UUID, typeID uuid.UUID, values Values, requireAll bool) (map[uuid.UUID]json.RawMessage, error) {
	t.Helper()
	ctx := context.Background()
	var out map[uuid.UUID]json.RawMessage
	err := f.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		defs, err := Load(ctx, q)
		if err != nil {
			return err
		}
		sc := Scope{Entity: entity, Enabled: map[uuid.UUID]bool{}, TypeID: typeID}
		for _, id := range enabled {
			sc.Enabled[id] = true
		}
		st := ProjectValues(q, f.project)
		if entity == EntityWorkItem {
			st = WorkItemValues(q, f.workItem)
		}
		out, err = Apply(ctx, defs, st, sc, values, requireAll)
		return err
	})
	return out, err
}

func code(err error) string {
	var e *apperr.Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

func TestValuesByTypeAndScope(t *testing.T) {
	f := newFixture(t)
	budget := f.field(t, fieldRequest{Entity: EntityProject, Name: "Presupuesto", Type: fields.TypeNumber, Required: true})
	sponsor := f.field(t, fieldRequest{Entity: EntityProject, Name: "Patrocinador", Type: fields.TypeText})
	severity := f.field(t, fieldRequest{Entity: EntityWorkItem, Name: "Severidad", Type: fields.TypeSelect,
		Options: []fields.Option{{Value: "Alta"}, {Value: "Baja"}}, TypeIDs: []uuid.UUID{f.bug}})
	enabled := []uuid.UUID{budget.ID, severity.ID}

	// Project: wrong type, not enabled, missing required, then valid (RF-13, RF-15, RF-16).
	if _, err := f.apply(t, EntityProject, enabled, uuid.Nil, Values{budget.ID: json.RawMessage(`"mucho"`)}, true); code(err) != "custom_field_invalid" {
		t.Fatalf("number as string: %v", err)
	}
	if _, err := f.apply(t, EntityProject, enabled, uuid.Nil, Values{sponsor.ID: json.RawMessage(`"Ana"`), budget.ID: json.RawMessage(`1`)}, true); code(err) != "custom_field_not_enabled" {
		t.Fatalf("field not enabled: %v", err)
	}
	if _, err := f.apply(t, EntityProject, enabled, uuid.Nil, Values{}, true); code(err) != "custom_field_required" {
		t.Fatalf("missing required: %v", err)
	}
	if _, err := f.apply(t, EntityProject, enabled, uuid.Nil, Values{severity.ID: json.RawMessage(`"Alta"`), budget.ID: json.RawMessage(`1`)}, true); code(err) != "custom_field_not_enabled" {
		t.Fatalf("work item field on a project: %v", err)
	}
	got, err := f.apply(t, EntityProject, enabled, uuid.Nil, Values{budget.ID: json.RawMessage(`1500.5`)}, true)
	if err != nil || string(got[budget.ID]) != "1500.5" {
		t.Fatalf("valid value = %s, %v", got[budget.ID], err)
	}

	// Work item: limited to Bug (RF-14), option must exist.
	if _, err := f.apply(t, EntityWorkItem, enabled, f.task, Values{severity.ID: json.RawMessage(`"Alta"`)}, true); code(err) != "custom_field_not_enabled" {
		t.Fatalf("field on a type it is not limited to: %v", err)
	}
	if _, err := f.apply(t, EntityWorkItem, enabled, f.bug, Values{severity.ID: json.RawMessage(`"Media"`)}, true); code(err) != "custom_field_invalid" {
		t.Fatalf("unknown option: %v", err)
	}
	if _, err := f.apply(t, EntityWorkItem, enabled, f.bug, Values{severity.ID: json.RawMessage(`"Alta"`)}, true); err != nil {
		t.Fatalf("valid option: %v", err)
	}
}

func TestRequiredLaterAndRetirement(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	area := f.field(t, fieldRequest{Entity: EntityProject, Name: "Área", Type: fields.TypeText})
	enabled := []uuid.UUID{area.ID}
	if _, err := f.apply(t, EntityProject, enabled, uuid.Nil, Values{area.ID: json.RawMessage(`"BI"`)}, true); err != nil {
		t.Fatal(err)
	}

	// The type is locked once there are values.
	if _, err := f.svc.Update(ctx, f.admin, area.ID, fieldRequest{Name: "Área", Type: fields.TypeNumber}); code(err) != "custom_field_type_locked" {
		t.Fatalf("type change with values: %v", err)
	}
	// Making it required does not touch records until their next edit (RF-17).
	if _, err := f.svc.Update(ctx, f.admin, area.ID, fieldRequest{Name: "Área", Type: fields.TypeText, Required: true}); err != nil {
		t.Fatal(err)
	}

	// Retired: values kept, read-only, not required.
	if _, err := f.svc.SetRetired(ctx, f.admin, area.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := f.apply(t, EntityProject, enabled, uuid.Nil, Values{area.ID: json.RawMessage(`"Otra"`)}, true); code(err) != "custom_field_retired" {
		t.Fatalf("editing a retired value: %v", err)
	}
	got, err := f.apply(t, EntityProject, enabled, uuid.Nil, Values{}, true)
	if err != nil || string(got[area.ID]) != `"BI"` {
		t.Fatalf("retired value = %s, %v; want kept and not required", got[area.ID], err)
	}

	// Duplicate names among active fields of the same entity.
	f.field(t, fieldRequest{Entity: EntityProject, Name: "Área", Type: fields.TypeText})
	if _, err := f.svc.SetRetired(ctx, f.admin, area.ID, false); code(err) != "catalog_name_conflict" {
		t.Fatalf("restore with a clash: %v", err)
	}
	if _, err := f.svc.Create(ctx, f.admin, fieldRequest{Entity: EntityWorkItem, Name: "área", Type: fields.TypeText}); err != nil {
		t.Fatalf("same name on another entity: %v", err)
	}
	if _, err := f.svc.Create(ctx, f.admin, fieldRequest{Entity: EntityProject, Name: "Tipos", Type: fields.TypeText, TypeIDs: []uuid.UUID{f.bug}}); code(err) != "validation_failed" {
		t.Fatalf("type limit on a project field: %v", err)
	}
	if _, err := f.svc.Create(ctx, f.admin, fieldRequest{Entity: EntityWorkItem, Name: "Sel", Type: fields.TypeSelect}); code(err) != "option_required" {
		t.Fatalf("select without options: %v", err)
	}
}
