package apitest

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

type catalogDTO struct {
	Methodologies []struct {
		ID     uuid.UUID `json:"id"`
		Name   string    `json:"name"`
		Phases []struct {
			Name         string `json:"name"`
			HasStartGate bool   `json:"has_start_gate"`
		} `json:"phases"`
	} `json:"methodologies"`
	Types []struct {
		ID      uuid.UUID `json:"id"`
		Name    string    `json:"name"`
		Retired bool      `json:"retired"`
	} `json:"work_item_types"`
	Statuses []struct {
		ID        uuid.UUID `json:"id"`
		Name      string    `json:"name"`
		IsDefault bool      `json:"is_default"`
		IsClosed  bool      `json:"is_closed"`
	} `json:"statuses"`
	Priorities []struct {
		Name      string `json:"name"`
		IsDefault bool   `json:"is_default"`
	} `json:"priorities"`
}

type idDTO struct {
	ID uuid.UUID `json:"id"`
}

func TestCatalogSeedAndRead(t *testing.T) {
	env := New(t)
	viewer := env.As(env.CreateUser(UserSpec{Email: "v@x.com", Roles: []string{"viewer"}}))
	var c catalogDTO
	viewer.Get("/api/catalog").Expect(t, http.StatusOK).JSON(t, &c)
	if len(c.Methodologies) != 2 || len(c.Types) != 6 || len(c.Statuses) != 6 || len(c.Priorities) != 4 {
		t.Fatalf("seed catalog = %+v (RF-11)", c)
	}
	for _, m := range c.Methodologies {
		if m.Name == "PRINCE2" && (len(m.Phases) != 4 || !m.Phases[0].HasStartGate) {
			t.Fatalf("PRINCE2 phases = %+v", m.Phases)
		}
	}
	viewer.Post("/api/statuses", map[string]any{"name": "X", "color": "#000000"}).Expect(t, http.StatusForbidden, "forbidden")
}

func TestCatalogNamesAndRetirement(t *testing.T) {
	env := New(t)
	admin := env.Admin()

	var typ idDTO
	admin.Post("/api/work-item-types", map[string]any{"name": " Riesgo ", "color": "#ABCDEF", "position": 7}).
		Expect(t, http.StatusCreated).JSON(t, &typ)
	admin.Post("/api/work-item-types", map[string]any{"name": "riesgo", "color": "#000000"}).Expect(t, http.StatusConflict, "catalog_name_conflict")
	admin.Post("/api/work-item-types", map[string]any{"name": "Bad", "color": "red"}).Expect(t, http.StatusBadRequest, "validation_failed")
	admin.Post("/api/work-item-types/"+typ.ID.String()+"/retire", nil).Expect(t, http.StatusOK)
	// The name is free again while retired; restoring then clashes (RF-8, RF-9).
	admin.Post("/api/work-item-types", map[string]any{"name": "Riesgo", "color": "#000000"}).Expect(t, http.StatusCreated)
	admin.Post("/api/work-item-types/"+typ.ID.String()+"/restore", nil).Expect(t, http.StatusConflict, "catalog_name_conflict")

	var m idDTO
	admin.Post("/api/methodologies", map[string]any{"name": "PM²"}).Expect(t, http.StatusCreated).JSON(t, &m)
	admin.Post("/api/methodologies/"+m.ID.String()+"/phases", map[string]any{"name": "Inicio", "has_start_gate": true}).Expect(t, http.StatusCreated)
	admin.Post("/api/methodologies/"+uuid.NewString()+"/phases", map[string]any{"name": "X"}).Expect(t, http.StatusNotFound, "not_found")

	if got := env.AuditActions(typ.ID.String()); len(got) != 2 || got[0] != "catalog.work_item_type.created" || got[1] != "catalog.work_item_type.retired" {
		t.Fatalf("audit = %v", got)
	}
}

func TestCatalogDefaults(t *testing.T) {
	env := New(t)
	admin := env.Admin()
	nuevo := env.CatalogID("statuses", "Nuevo")
	enProgreso := env.CatalogID("statuses", "En progreso")

	// The default can be moved, never switched off or retired (RF-5, RF-10).
	admin.Do(http.MethodPatch, "/api/statuses/"+nuevo.String(), map[string]any{"name": "Nuevo", "color": "#3b82f6", "is_default": false}).
		Expect(t, http.StatusConflict, "catalog_default_required")
	admin.Post("/api/statuses/"+nuevo.String()+"/retire", nil).Expect(t, http.StatusConflict, "catalog_default_required")
	admin.Do(http.MethodPatch, "/api/statuses/"+enProgreso.String(), map[string]any{"name": "En progreso", "color": "#f59e0b", "is_default": true}).
		Expect(t, http.StatusOK)
	admin.Post("/api/statuses", map[string]any{"name": "Raro", "color": "#000000", "is_default": true, "is_closed": true}).
		Expect(t, http.StatusBadRequest, "validation_failed")

	var c catalogDTO
	admin.Get("/api/catalog").Expect(t, http.StatusOK).JSON(t, &c)
	defaults := 0
	for _, s := range c.Statuses {
		if s.IsDefault {
			defaults++
			if s.ID != enProgreso {
				t.Fatalf("default = %s, want En progreso", s.Name)
			}
		}
	}
	if defaults != 1 {
		t.Fatalf("defaults = %d, want 1", defaults)
	}

	normal := env.CatalogID("priorities", "Normal")
	admin.Post("/api/priorities/"+normal.String()+"/retire", nil).Expect(t, http.StatusConflict, "catalog_default_required")
	admin.Post("/api/priorities", map[string]any{"name": "Crítica", "color": "#000000", "is_default": true}).Expect(t, http.StatusCreated)
	admin.Post("/api/priorities/"+normal.String()+"/retire", nil).Expect(t, http.StatusOK)
}

func TestWorkflowTransitions(t *testing.T) {
	env := New(t)
	admin := env.Admin()
	bug := env.CatalogID("work_item_types", "Bug")
	nuevo := env.CatalogID("statuses", "Nuevo")
	prog := env.CatalogID("statuses", "En progreso")
	cerrado := env.CatalogID("statuses", "Cerrado")

	path := "/api/work-item-types/" + bug.String() + "/transitions"
	body := map[string]any{"transitions": []map[string]any{
		{"from_status_id": nuevo, "to_status_id": prog},
		{"from_status_id": prog, "to_status_id": cerrado, "role": "lead"},
		{"from_status_id": nuevo, "to_status_id": prog},
	}}
	var res struct {
		Transitions []struct {
			Role *string `json:"role"`
		} `json:"transitions"`
	}
	admin.Do(http.MethodPut, path, body).Expect(t, http.StatusOK).JSON(t, &res)
	if len(res.Transitions) != 2 {
		t.Fatalf("transitions = %+v, want duplicates merged", res.Transitions)
	}
	admin.Do(http.MethodPut, path, map[string]any{"transitions": []map[string]any{{"from_status_id": nuevo, "to_status_id": nuevo}}}).
		Expect(t, http.StatusBadRequest, "validation_failed")
	admin.Do(http.MethodPut, path, map[string]any{"transitions": []map[string]any{{"from_status_id": nuevo, "to_status_id": prog, "role": "boss"}}}).
		Expect(t, http.StatusBadRequest, "validation_failed")

	member := env.As(env.CreateUser(UserSpec{Email: "m@x.com", Roles: []string{"member"}}))
	member.Get(path).Expect(t, http.StatusOK)
	member.Do(http.MethodPut, path, body).Expect(t, http.StatusForbidden, "forbidden")
}

func TestStatusClosedFlagLockedOnceUsed(t *testing.T) {
	env := New(t)
	admin := env.Admin()
	owner := env.CreateUser(UserSpec{Email: "o@x.com", Roles: []string{"member"}})
	prog := env.CatalogID("statuses", "En progreso")
	body := map[string]any{"name": "En progreso", "color": "#f59e0b", "is_closed": true}

	// Unused: the flag can change.
	admin.Do(http.MethodPatch, "/api/statuses/"+prog.String(), body).Expect(t, http.StatusOK)
	body["is_closed"] = false
	admin.Do(http.MethodPatch, "/api/statuses/"+prog.String(), body).Expect(t, http.StatusOK)

	env.SeedWorkItem(env.SeedProject("KAI", env.CreateTeam("BI", nil), owner), prog, owner)
	body["is_closed"] = true
	admin.Do(http.MethodPatch, "/api/statuses/"+prog.String(), body).Expect(t, http.StatusConflict, "status_closed_locked")
	body["is_closed"] = false
	body["name"] = "Haciendo"
	admin.Do(http.MethodPatch, "/api/statuses/"+prog.String(), body).Expect(t, http.StatusOK)
}
