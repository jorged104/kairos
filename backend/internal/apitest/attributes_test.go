package apitest

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/attributes"
	"openscum/internal/db/sqlc"
)

type attrDTO struct {
	ID        uuid.UUID           `json:"id"`
	Name      string              `json:"name"`
	Type      string              `json:"type"`
	Required  bool                `json:"required"`
	Options   []attributes.Option `json:"options"`
	RetiredAt *string             `json:"retired_at"`
}

func createAttr(t *testing.T, c *Client, body map[string]any) attrDTO {
	t.Helper()
	var a attrDTO
	c.Post("/api/attributes", body).Expect(t, http.StatusCreated).JSON(t, &a)
	return a
}

func TestDefineAttributes(t *testing.T) {
	env := New(t)
	admin := env.As(env.CreateUser(UserSpec{Email: "admin@x.com", Roles: []string{"admin"}}))

	for _, typ := range []string{"text", "number", "date", "boolean"} {
		a := createAttr(t, admin, map[string]any{"name": "A " + typ, "type": typ})
		if a.Type != typ || a.Required {
			t.Fatalf("attribute = %+v", a)
		}
	}
	admin.Post("/api/attributes", map[string]any{"name": "Sel", "type": "select"}).Expect(t, http.StatusBadRequest, "option_required")
	sel := createAttr(t, admin, map[string]any{"name": "Sel", "type": "select", "required": true, "options": []map[string]any{{"value": "x"}}})
	if !sel.Required || len(sel.Options) != 1 {
		t.Fatalf("select = %+v", sel)
	}
	admin.Post("/api/attributes", map[string]any{"name": " sel ", "type": "text"}).Expect(t, http.StatusConflict, "attribute_name_conflict")
	admin.Post("/api/attributes", map[string]any{"name": "Bad", "type": "color"}).Expect(t, http.StatusBadRequest, "validation_failed")
}

func TestAttributeTypeLockedOnceItHasValues(t *testing.T) {
	env := New(t)
	adminID := env.CreateUser(UserSpec{Email: "admin@x.com", Roles: []string{"admin"}})
	admin := env.As(adminID)
	a := createAttr(t, admin, map[string]any{"name": "Ingreso", "type": "text"})
	path := "/api/attributes/" + a.ID.String()

	admin.Do(http.MethodPatch, path, map[string]any{"name": "Ingreso", "type": "date"}).Expect(t, http.StatusOK)
	setValue(t, env, adminID, a.ID, `"2026-01-01"`)
	admin.Do(http.MethodPatch, path, map[string]any{"name": "Ingreso", "type": "text"}).Expect(t, http.StatusConflict, "attribute_type_locked")
	admin.Do(http.MethodPatch, path, map[string]any{"name": "Fecha de ingreso", "type": "date", "required": true}).Expect(t, http.StatusOK)
}

func TestRetireAndRestoreAttribute(t *testing.T) {
	env := New(t)
	adminID := env.CreateUser(UserSpec{Email: "admin@x.com", Roles: []string{"admin"}})
	member := env.As(env.CreateUser(UserSpec{Email: "m@x.com", Roles: []string{"member"}}))
	admin := env.As(adminID)
	a := createAttr(t, admin, map[string]any{"name": "Salida", "type": "date"})
	setValue(t, env, adminID, a.ID, `"2026-01-01"`)

	admin.Post("/api/attributes/"+a.ID.String()+"/retire", nil).Expect(t, http.StatusOK)
	// Values are kept (RF-103) and only admins see the retired attribute (RF-105).
	if v := valueOf(t, env, adminID, a.ID); v != `"2026-01-01"` {
		t.Fatalf("value after retire = %s", v)
	}
	var list struct{ Items []attrDTO }
	member.Get("/api/attributes").JSON(t, &list)
	if len(list.Items) != 0 {
		t.Fatalf("member sees retired attribute: %+v", list.Items)
	}
	admin.Get("/api/attributes").JSON(t, &list)
	if len(list.Items) != 1 || list.Items[0].RetiredAt == nil {
		t.Fatalf("admin list = %+v", list.Items)
	}

	// A new active attribute may reuse the name; restoring then clashes.
	other := createAttr(t, admin, map[string]any{"name": "Salida", "type": "text"})
	admin.Post("/api/attributes/"+a.ID.String()+"/restore", nil).Expect(t, http.StatusConflict, "attribute_name_conflict")
	admin.Post("/api/attributes/"+other.ID.String()+"/retire", nil).Expect(t, http.StatusOK)
	admin.Post("/api/attributes/"+a.ID.String()+"/restore", nil).Expect(t, http.StatusOK)
	if got := env.AuditActions(a.ID.String()); len(got) != 3 {
		t.Fatalf("audit = %v", got)
	}
}

func TestRetiredSelectOptionIsKeptButNotChoosable(t *testing.T) {
	env := New(t)
	adminID := env.CreateUser(UserSpec{Email: "admin@x.com", Roles: []string{"admin"}})
	other := env.CreateUser(UserSpec{Email: "o@x.com", Roles: []string{"member"}})
	admin := env.As(adminID)
	a := createAttr(t, admin, map[string]any{"name": "Sede", "type": "select", "options": []map[string]any{{"value": "Norte"}, {"value": "Sur"}}})
	setValue(t, env, adminID, a.ID, `"Norte"`)

	var updated attrDTO
	admin.Do(http.MethodPatch, "/api/attributes/"+a.ID.String(), map[string]any{"name": "Sede", "type": "select", "options": []map[string]any{{"value": "Sur"}}}).
		Expect(t, http.StatusOK).JSON(t, &updated)
	if len(updated.Options) != 2 || !updated.Options[1].Retired {
		t.Fatalf("options = %+v", updated.Options)
	}
	if v := valueOf(t, env, adminID, a.ID); v != `"Norte"` {
		t.Fatalf("existing value lost: %s", v)
	}
	if err := apply(env, other, a.ID, `"Norte"`); err == nil {
		t.Fatal("retired option chosen again")
	}
	if err := apply(env, adminID, a.ID, `"Norte"`); err != nil {
		t.Fatalf("user can't keep its retired option: %v", err)
	}
}

func TestAttributeDefinitionIsAdminOnly(t *testing.T) {
	env := New(t)
	lead := env.As(env.CreateUser(UserSpec{Email: "l@x.com", Roles: []string{"lead"}}))
	lead.Post("/api/attributes", map[string]any{"name": "X", "type": "text"}).Expect(t, http.StatusForbidden, "forbidden")
	lead.Get("/api/attributes").Expect(t, http.StatusOK)
}

func apply(env *Env, user, attr uuid.UUID, raw string) error {
	return env.Store.InTx(context.Background(), func(_ pgx.Tx, q *sqlc.Queries) error {
		return attributes.Apply(context.Background(), q, user, attributes.Values{attr: json.RawMessage(raw)}, false)
	})
}

func setValue(t *testing.T, env *Env, user, attr uuid.UUID, raw string) {
	t.Helper()
	if err := apply(env, user, attr, raw); err != nil {
		t.Fatal(err)
	}
}

func valueOf(t *testing.T, env *Env, user, attr uuid.UUID) string {
	t.Helper()
	rows, err := env.Store.ListUserAttributeValues(context.Background(), user)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.AttributeID == attr {
			return string(r.Value)
		}
	}
	return ""
}
