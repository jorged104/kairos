package apitest

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
)

type pageDTO struct {
	Items    []userDTO `json:"items"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
}

func dir(t *testing.T, c *Client, q url.Values) pageDTO {
	t.Helper()
	var p pageDTO
	c.Get("/api/users?"+q.Encode()).Expect(t, http.StatusOK).JSON(t, &p)
	return p
}

func names(p pageDTO) []string {
	out := make([]string, len(p.Items))
	for i, u := range p.Items {
		out[i] = u.Name
	}
	return out
}

func TestDirectoryPagingAndOrder(t *testing.T) {
	env := New(t)
	viewer := env.CreateUser(UserSpec{Name: "aaa viewer", Email: "v@x.com", Roles: []string{"viewer"}})
	for i := range 30 {
		env.CreateUser(UserSpec{Name: fmt.Sprintf("Usuario %02d", 29-i), Email: fmt.Sprintf("u%d@x.com", i)})
	}
	c := env.As(viewer)
	p := dir(t, c, nil)
	if p.Total != 31 || len(p.Items) != 25 || p.PageSize != 25 || p.Items[1].Name != "Usuario 00" {
		t.Fatalf("page = total %d, %d items, first %v", p.Total, len(p.Items), names(p)[:2])
	}
	p = dir(t, c, url.Values{"page": {"2"}, "page_size": {"25"}})
	if len(p.Items) != 6 || p.Items[5].Name != "Usuario 29" {
		t.Fatalf("page 2 = %v", names(p))
	}
	c.Get("/api/users?page_size=101").Expect(t, http.StatusBadRequest, "invalid_filter")
	if p := dir(t, c, url.Values{"page_size": {"100"}}); len(p.Items) != 31 {
		t.Fatalf("page_size 100 = %d items", len(p.Items))
	}
}

func TestDirectorySearchIgnoresCaseAndAccents(t *testing.T) {
	f := newFixture(t)
	f.env.CreateUser(UserSpec{Name: "José Pérez", Email: "jperez@x.com"})
	c := f.env.As(f.admin)
	for _, q := range []string{"jose", "PEREZ", "pérez"} {
		if p := dir(t, c, url.Values{"q": {q}}); p.Total != 1 || p.Items[0].Name != "José Pérez" {
			t.Fatalf("q=%s: %v", q, names(p))
		}
	}
	if p := dir(t, c, url.Values{"q": {"JPEREZ@"}}); p.Total != 1 {
		t.Fatalf("admin email search = %v", names(p))
	}
	// A member cannot search by someone else's email (RF-115).
	if p := dir(t, f.env.As(f.member), url.Values{"q": {"jperez@"}}); p.Total != 0 {
		t.Fatalf("member email search = %v", names(p))
	}
	if p := dir(t, c, url.Values{"q": {"%"}}); p.Total != 0 {
		t.Fatalf("q=%% matched %v", names(p))
	}
}

func TestDirectoryFilters(t *testing.T) {
	f := newFixture(t)
	admin := f.env.As(f.admin)
	ingreso := createAttr(t, admin, map[string]any{"name": "Ingreso", "type": "date"})
	horas := createAttr(t, admin, map[string]any{"name": "Horas", "type": "number"})
	remoto := createAttr(t, admin, map[string]any{"name": "Remoto", "type": "boolean"})
	sede := createAttr(t, admin, map[string]any{"name": "Sede", "type": "select", "options": []map[string]any{{"value": "Norte"}, {"value": "Sur"}}})
	set := func(id uuid.UUID, vals map[string]any) {
		admin.Do(http.MethodPatch, "/api/users/"+id.String(), map[string]any{"attributes": vals}).Expect(t, http.StatusOK)
	}
	set(f.member, map[string]any{ingreso.ID.String(): "2024-03-01", horas.ID.String(): 40, remoto.ID.String(): true, sede.ID.String(): "Norte"})
	set(f.outsider, map[string]any{ingreso.ID.String(): "2026-01-15", horas.ID.String(): 20, remoto.ID.String(): false, sede.ID.String(): "Sur"})

	cases := []struct {
		q    url.Values
		want []string
	}{
		{url.Values{"team_id": {f.bi.String()}}, []string{"Member", "Otro lead"}}, // includes BI Reportes
		{url.Values{"team_id": {f.biRep.String()}}, []string{"Member"}},
		{url.Values{"role": {"lead"}}, []string{"Lead", "Otro lead"}},
		{url.Values{"role": {"member"}, "team_id": {f.other.String()}}, []string{"Outsider"}},
		{url.Values{"attr." + ingreso.ID.String() + ".from": {"2025-01-01"}}, []string{"Outsider"}},
		{url.Values{"attr." + ingreso.ID.String() + ".to": {"2025-01-01"}}, []string{"Member"}},
		{url.Values{"attr." + horas.ID.String() + ".from": {"20"}, "attr." + horas.ID.String() + ".to": {"30"}}, []string{"Outsider"}},
		{url.Values{"attr." + remoto.ID.String(): {"true"}}, []string{"Member"}},
		{url.Values{"attr." + sede.ID.String(): {"Sur"}}, []string{"Outsider"}},
		{url.Values{"status": {"inactive"}}, nil},
	}
	for _, c := range cases {
		got := names(dir(t, admin, c.q))
		if fmt.Sprint(got) != fmt.Sprint(c.want) && !(len(got) == 0 && len(c.want) == 0) {
			t.Errorf("%v = %v, want %v", c.q, got, c.want)
		}
	}

	for _, q := range []string{"team_id=" + uuid.NewString(), "team_id=x", "role=owner", "status=gone",
		"attr." + uuid.NewString() + "=x", "attr." + horas.ID.String() + ".from=abc", "attr." + remoto.ID.String() + "=yes"} {
		admin.Get("/api/users?"+q).Expect(t, http.StatusBadRequest, "invalid_filter")
	}
}

func TestDirectoryHidesOffboardedUnlessFiltered(t *testing.T) {
	f := newFixture(t)
	admin := f.env.As(f.admin)
	admin.Post("/api/users/"+f.outsider.String()+"/offboard", nil).Expect(t, http.StatusOK)
	for _, n := range names(dir(t, admin, nil)) {
		if n == "Outsider" {
			t.Fatal("offboarded user listed by default (RF-113)")
		}
	}
	if got := names(dir(t, admin, url.Values{"status": {"offboarded"}})); len(got) != 1 || got[0] != "Outsider" {
		t.Fatalf("offboarded filter = %v", got)
	}
}

func TestDirectoryFieldsByViewer(t *testing.T) {
	f := newFixture(t)
	admin := f.env.As(f.admin)
	attr := createAttr(t, admin, map[string]any{"name": "Sede", "type": "text"})
	admin.Do(http.MethodPatch, "/api/users/"+f.outsider.String(), map[string]any{"attributes": map[string]any{attr.ID.String(): "Norte"}}).Expect(t, http.StatusOK)

	byName := func(p pageDTO) map[string]userDTO {
		out := map[string]userDTO{}
		for _, u := range p.Items {
			out[u.Name] = u
		}
		return out
	}
	lead := byName(dir(t, f.env.As(f.lead), nil))
	if lead["Member"].Email == nil || lead["Outsider"].Email != nil || lead["Otro lead"].Email != nil || lead["Lead"].Email == nil {
		t.Fatalf("lead view: %+v", lead)
	}
	if len(lead["Outsider"].Teams) != 1 {
		t.Fatalf("basic team field missing: %+v", lead["Outsider"])
	}
	// The lead can't filter by attributes or roles of users outside its scope.
	if got := names(dir(t, f.env.As(f.lead), url.Values{"attr." + attr.ID.String(): {"Norte"}})); len(got) != 0 {
		t.Fatalf("lead filtered outside its scope: %v", got)
	}
	if got := names(dir(t, f.env.As(f.member), url.Values{"role": {"member"}})); len(got) != 1 || got[0] != "Member" {
		t.Fatalf("member role filter = %v", got)
	}

	admin.Post("/api/attributes/"+attr.ID.String()+"/retire", nil).Expect(t, http.StatusOK)
	f.env.As(f.lead).Get("/api/users?attr."+attr.ID.String()+"=Norte").Expect(t, http.StatusBadRequest, "invalid_filter")
	if got := names(dir(t, admin, url.Values{"attr." + attr.ID.String(): {"Norte"}})); len(got) != 1 {
		t.Fatalf("admin retired filter = %v", got)
	}
}

func TestDirectoryPerformanceAt500Users(t *testing.T) {
	env := New(t)
	ctx := context.Background()
	admin := env.CreateUser(UserSpec{Name: "Admin", Email: "admin@x.com", Roles: []string{"admin"}})
	team := env.CreateTeam("T", nil)
	exec := func(sql string, args ...any) {
		if _, err := env.Store.Pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO attributes (name, name_normalized, type) SELECT 'A'||g, 'a'||g, 'text' FROM generate_series(1, 50) g`)
	exec(`INSERT INTO users (name, email, email_normalized) SELECT 'Persona '||g, 'p'||g||'@x.com', 'p'||g||'@x.com' FROM generate_series(1, 500) g`)
	exec(`INSERT INTO user_roles (user_id, role, source) SELECT id, 'member', 'manual' FROM users WHERE email LIKE 'p%'`)
	exec(`INSERT INTO team_members (team_id, user_id, source) SELECT $1, id, 'manual' FROM users WHERE email LIKE 'p%'`, team)
	exec(`INSERT INTO user_attribute_values (user_id, attribute_id, value) SELECT u.id, a.id, to_jsonb('v'||a.name) FROM users u, attributes a WHERE u.email LIKE 'p%'`)
	exec(`ANALYZE`)

	c := env.As(admin)
	c.Get("/api/users?page_size=100").Expect(t, http.StatusOK) // warm up
	start := time.Now()
	c.Get("/api/users?page_size=100&q=persona&team_id="+team.String()).Expect(t, http.StatusOK)
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Fatalf("directory took %v, want < 500 ms", d)
	}
}
