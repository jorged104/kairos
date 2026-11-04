package apitest

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/google/uuid"
)

type levelDTO struct {
	ID     uuid.UUID `json:"id"`
	Number int       `json:"number"`
	Name   string    `json:"name"`
	MinXP  int       `json:"min_xp"`
}

type pointsDTO struct {
	XP      int64     `json:"xp"`
	Coins   int64     `json:"coins"`
	Level   levelDTO  `json:"level"`
	Next    *levelDTO `json:"next"`
	LastSeq int64     `json:"last_seq"`
}

type movementsDTO struct {
	Items []struct {
		Seq      int64   `json:"seq"`
		XP       int     `json:"xp"`
		Coins    int     `json:"coins"`
		Reason   string  `json:"reason"`
		Note     *string `json:"note"`
		WorkItem *struct {
			Key   string `json:"key"`
			Title string `json:"title"`
		} `json:"work_item"`
	} `json:"items"`
	Total int64 `json:"total"`
}

func points(t *testing.T, c *Client) pointsDTO {
	t.Helper()
	var out pointsDTO
	c.Get("/api/me/points").Expect(t, http.StatusOK).JSON(t, &out)
	return out
}

func movements(t *testing.T, c *Client, query string) movementsDTO {
	t.Helper()
	var out movementsDTO
	c.Get("/api/me/points/movements"+query).Expect(t, http.StatusOK).JSON(t, &out)
	return out
}

// setStatus moves a work item keeping its assignees and size unless changed.
func (s *workSetup) moveItem(t *testing.T, id uuid.UUID, status uuid.UUID, changes map[string]any) {
	t.Helper()
	body := map[string]any{"status_id": status}
	for k, v := range changes {
		body[k] = v
	}
	if _, ok := body["assignee_ids"]; !ok {
		body["assignee_ids"] = s.assignees(t, id)
	}
	if _, ok := body["size_id"]; !ok {
		body["size_id"] = s.sized(t, id).SizeID
	}
	s.patch(s.admin, s.get(t, id), body).Expect(t, http.StatusOK)
	s.env.App.Events.Poll(t.Context())
}

func (s *workSetup) assignees(t *testing.T, id uuid.UUID) []uuid.UUID {
	t.Helper()
	rows, err := s.env.Store.Pool.Query(t.Context(), "SELECT user_id FROM work_item_assignees WHERE work_item_id = $1", id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := []uuid.UUID{}
	for rows.Next() {
		var u uuid.UUID
		if err := rows.Scan(&u); err != nil {
			t.Fatal(err)
		}
		out = append(out, u)
	}
	return out
}

func TestPointsForClosingAndReopening(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	nuevo, cerrado, resuelto := s.env.CatalogID("statuses", "Nuevo"), s.env.CatalogID("statuses", "Cerrado"), s.env.CatalogID("statuses", "Resuelto")
	m, xl := s.env.CatalogID("work_item_sizes", "M"), s.env.CatalogID("work_item_sizes", "XL")
	both := []uuid.UUID{s.memberID, s.outsiderID}

	// Everybody starts at 0: nothing closed before counts (RF-53).
	early := s.item(t, s.member, project, map[string]any{"title": "Antes", "assignee_ids": []uuid.UUID{s.memberID}})
	if got := points(t, s.member); got.XP != 0 || got.Coins != 0 || got.Level.Number != 1 || got.Level.Name != "Nivel 1" || got.Next == nil || got.Next.MinXP != 100 {
		t.Fatalf("start = %+v (RF-34, RF-37)", got)
	}

	// Each assignee gets the full value of the size (RF-6).
	wi := s.item(t, s.member, project, map[string]any{"title": "Informe", "assignee_ids": both, "size_id": m})
	s.moveItem(t, wi.ID, cerrado, nil)
	for _, c := range []*Client{s.member, s.outsider} {
		if got := points(t, c); got.XP != 20 || got.Coins != 4 {
			t.Fatalf("after close = %+v, want 20 XP and 4 coins (RF-6, RF-27)", got)
		}
	}
	h := movements(t, s.member, "")
	if h.Total != 1 || h.Items[0].Reason != "close" || h.Items[0].WorkItem == nil || h.Items[0].WorkItem.Key != wi.Key {
		t.Fatalf("history = %+v (RF-39)", h)
	}

	// A repeated delivery pays nothing more (RF-12), nor does a change between closed states (RF-17).
	s.exec(t, "UPDATE event_consumers SET last_event_id = 0 WHERE name = 'gamification'")
	s.env.App.Events.Poll(t.Context())
	s.moveItem(t, wi.ID, resuelto, nil)
	if got := points(t, s.member); got.XP != 20 {
		t.Fatalf("after replay = %+v (RF-12, RF-17)", got)
	}

	// The table changes, then the reopening takes back what was given (RF-11, RF-14).
	s.admin.Do(http.MethodPut, "/api/gamification/values", map[string]any{"no_size": map[string]any{"xp": 5, "coins": 1},
		"sizes": []map[string]any{{"size_id": m, "xp": 100, "coins": 0}}}).Expect(t, http.StatusOK)
	s.moveItem(t, wi.ID, nuevo, nil)
	if got := points(t, s.outsider); got.XP != 0 || got.Coins != 0 {
		t.Fatalf("after reopen = %+v (RF-14)", got)
	}
	// Closed again with other assignees and size: the new ones are paid (RF-15).
	s.moveItem(t, wi.ID, cerrado, map[string]any{"assignee_ids": []uuid.UUID{s.memberID}, "size_id": xl})
	if a, b := points(t, s.member), points(t, s.outsider); a.XP != 80 || a.Coins != 16 || b.XP != 0 {
		t.Fatalf("after second close = %+v / %+v (RF-15)", a, b)
	}
	// Reopening a close that paid nobody writes nothing (RF-16)...
	s.moveItem(t, wi.ID, nuevo, nil)
	s.moveItem(t, wi.ID, cerrado, map[string]any{"assignee_ids": []uuid.UUID{}})
	s.moveItem(t, wi.ID, nuevo, nil)
	if got := points(t, s.member); got.XP != 0 {
		t.Fatalf("after empty cycle = %+v (RF-8, RF-16)", got)
	}
	// ...nor does reopening work closed before the start (RF-54).
	s.exec(t, "UPDATE work_items SET status_id = $1 WHERE id = $2", cerrado, early.ID)
	s.moveItem(t, early.ID, nuevo, nil)
	if got := movements(t, s.member, ""); got.Total != 4 {
		t.Fatalf("movements = %d, want close, reopen, close, reopen (RF-54)", got.Total)
	}

	// Without size, the «sin talla» value (RF-7); in a template, nothing (RF-9).
	plain := s.item(t, s.member, project, map[string]any{"assignee_ids": []uuid.UUID{s.memberID}})
	s.moveItem(t, plain.ID, cerrado, nil)
	if got := points(t, s.member); got.XP != 5 || got.Coins != 1 {
		t.Fatalf("no size = %+v (RF-7)", got)
	}
	tpl := s.project(t, s.lead, map[string]any{"identifier": "TPL", "name": "Plantilla"})
	tplItem := s.item(t, s.member, tpl.ID, map[string]any{"assignee_ids": []uuid.UUID{s.memberID}, "size_id": xl})
	s.exec(t, "UPDATE projects SET is_template = true WHERE id = $1", tpl.ID)
	s.moveItem(t, tplItem.ID, cerrado, nil)
	if got := points(t, s.member); got.XP != 5 {
		t.Fatalf("template = %+v (RF-9)", got)
	}

	// A size worth nothing writes nothing (RF-10).
	s.admin.Do(http.MethodPut, "/api/gamification/values", map[string]any{"no_size": map[string]any{"xp": 0, "coins": 0}}).Expect(t, http.StatusOK)
	zero := s.item(t, s.member, project, map[string]any{"assignee_ids": []uuid.UUID{s.memberID}})
	s.moveItem(t, zero.ID, cerrado, nil)
	if got := movements(t, s.member, ""); got.Total != 5 {
		t.Fatalf("movements = %d (RF-10)", got.Total)
	}
	// The cursor of the notice returns only the newer ones (RF-41).
	last := points(t, s.member).LastSeq
	if got := movements(t, s.member, "?after_seq=0"); len(got.Items) != 5 {
		t.Fatalf("after 0 = %d", len(got.Items))
	}
	if got := movements(t, s.member, "?after_seq="+strconv.FormatInt(last, 10)); len(got.Items) != 0 {
		t.Fatalf("after last = %d", len(got.Items))
	}

	// The ledger cannot be edited nor emptied (RF-2).
	if _, err := s.env.Store.Pool.Exec(t.Context(), "UPDATE point_ledger SET xp = 999"); err == nil {
		t.Fatal("update allowed (RF-2)")
	}
	if _, err := s.env.Store.Pool.Exec(t.Context(), "DELETE FROM point_ledger"); err == nil {
		t.Fatal("delete allowed (RF-2)")
	}
}

func TestPointAdjustmentsAndTables(t *testing.T) {
	s := newWorkSetup(t)
	adjust := func(c *Client, user uuid.UUID, body map[string]any) *Response {
		return c.Post("/api/gamification/users/"+user.String()+"/adjustments", body)
	}

	// Adjustments (RF-18 to RF-23).
	adjust(s.admin, s.memberID, map[string]any{"xp": 150, "coins": 3, "note": "  Migración de puntos  "}).Expect(t, http.StatusCreated)
	if got := points(t, s.member); got.XP != 150 || got.Coins != 3 || got.Level.Number != 2 || got.Next.MinXP != 300 {
		t.Fatalf("after adjust = %+v (RF-18, RF-29)", got)
	}
	if h := movements(t, s.member, ""); h.Items[0].Reason != "adjust" || h.Items[0].Note == nil || *h.Items[0].Note != "Migración de puntos" {
		t.Fatalf("history = %+v (RF-39)", h)
	}
	adjust(s.admin, s.memberID, map[string]any{"xp": 0, "coins": 0, "note": "nada"}).Expect(t, http.StatusBadRequest, "points_adjust_empty")
	adjust(s.admin, s.memberID, map[string]any{"xp": 1, "coins": 0, "note": "x"}).Expect(t, http.StatusBadRequest, "validation_failed")
	adjust(s.admin, s.memberID, map[string]any{"xp": -151, "coins": 0, "note": "demasiado"}).Expect(t, http.StatusConflict, "points_negative")
	adjust(s.admin, s.adminID, map[string]any{"xp": 10, "coins": 0, "note": "para mí"}).Expect(t, http.StatusForbidden, "points_adjust_self")
	adjust(s.lead, s.memberID, map[string]any{"xp": 10, "coins": 0, "note": "líder"}).Expect(t, http.StatusForbidden)
	adjust(s.admin, uuid.New(), map[string]any{"xp": 10, "coins": 0, "note": "nadie"}).Expect(t, http.StatusNotFound)
	if got := s.env.AuditActions(s.memberID.String()); len(got) == 0 || got[0] != "points.adjusted" {
		t.Fatalf("audit = %v (RF-22)", got)
	}

	// Only the manager sees somebody else's points (RF-40).
	s.admin.Get("/api/gamification/users/" + s.memberID.String() + "/points").Expect(t, http.StatusOK)
	s.admin.Get("/api/gamification/users/" + s.memberID.String() + "/movements").Expect(t, http.StatusOK)
	s.lead.Get("/api/gamification/users/" + s.memberID.String() + "/movements").Expect(t, http.StatusForbidden)

	// The table of points: retired sizes keep showing while they are worth something (RF-24 to RF-28).
	var vals struct {
		NoSize struct{ XP, Coins int } `json:"no_size"`
		Sizes  []struct {
			SizeID  uuid.UUID `json:"size_id"`
			Name    string    `json:"name"`
			Retired bool      `json:"retired"`
			XP      int       `json:"xp"`
			Coins   int       `json:"coins"`
		} `json:"sizes"`
	}
	s.member.Get("/api/gamification/values").Expect(t, http.StatusOK).JSON(t, &vals)
	if vals.NoSize.XP != 5 || len(vals.Sizes) != 5 || vals.Sizes[4].Name != "XL" || vals.Sizes[4].XP != 80 || vals.Sizes[4].Coins != 16 {
		t.Fatalf("values = %+v (RF-27)", vals)
	}
	xl := vals.Sizes[4].SizeID
	s.exec(t, "UPDATE work_item_sizes SET retired_at = now() WHERE id = $1", xl)
	s.exec(t, "INSERT INTO work_item_sizes (name, name_normalized, position) VALUES ('XXL', 'xxl', 6)")
	s.member.Get("/api/gamification/values").Expect(t, http.StatusOK).JSON(t, &vals)
	if len(vals.Sizes) != 6 || !vals.Sizes[4].Retired || vals.Sizes[5].Name != "XXL" || vals.Sizes[5].XP != 0 {
		t.Fatalf("values = %+v (RF-25, RF-26)", vals)
	}
	put := func(c *Client, body map[string]any) *Response { return c.Do(http.MethodPut, "/api/gamification/values", body) }
	put(s.member, map[string]any{"no_size": map[string]any{"xp": 1, "coins": 1}}).Expect(t, http.StatusForbidden)
	put(s.admin, map[string]any{"no_size": map[string]any{"xp": 1001, "coins": 1}}).Expect(t, http.StatusBadRequest, "validation_failed")
	put(s.admin, map[string]any{"no_size": map[string]any{"xp": 1, "coins": 1}, "sizes": []map[string]any{{"size_id": uuid.New(), "xp": 1, "coins": 1}}}).
		Expect(t, http.StatusBadRequest, "validation_failed")
	put(s.admin, map[string]any{"no_size": map[string]any{"xp": 2, "coins": 0}, "sizes": []map[string]any{{"size_id": xl, "xp": 0, "coins": 0}}}).Expect(t, http.StatusOK)
	s.member.Get("/api/gamification/values").Expect(t, http.StatusOK).JSON(t, &vals)
	if vals.NoSize.XP != 2 || len(vals.Sizes) != 5 {
		t.Fatalf("values = %+v (RF-24, RF-26)", vals)
	}
	if got := s.env.AuditActions("point_values"); len(got) != 1 || got[0] != "point_values.updated" {
		t.Fatalf("audit = %v (RF-28)", got)
	}

	// Levels (RF-30 to RF-36).
	var ls struct{ Items []levelDTO }
	s.member.Get("/api/gamification/levels").Expect(t, http.StatusOK).JSON(t, &ls)
	if len(ls.Items) != 10 || ls.Items[9].MinXP != 4500 || ls.Items[2].Number != 3 {
		t.Fatalf("levels = %+v (RF-34)", ls.Items)
	}
	first, second := ls.Items[0], ls.Items[1]
	var created levelDTO
	s.admin.Post("/api/gamification/levels", map[string]any{"name": " Aprendiz ", "min_xp": 50}).Expect(t, http.StatusCreated).JSON(t, &created)
	if created.Number != 2 || created.Name != "Aprendiz" {
		t.Fatalf("created = %+v (RF-33)", created)
	}
	if got := points(t, s.member); got.Level.Number != 3 || got.Level.Name != "Nivel 2" {
		t.Fatalf("level = %+v, recomputed without movements (RF-35)", got)
	}
	s.admin.Post("/api/gamification/levels", map[string]any{"name": "Otro", "min_xp": 100}).Expect(t, http.StatusConflict, "level_conflict")
	s.admin.Post("/api/gamification/levels", map[string]any{"name": "", "min_xp": 7}).Expect(t, http.StatusBadRequest, "validation_failed")
	s.admin.Do(http.MethodPatch, "/api/gamification/levels/"+first.ID.String(), map[string]any{"name": "Inicio", "min_xp": 10}).Expect(t, http.StatusConflict, "level_locked")
	s.admin.Do(http.MethodPatch, "/api/gamification/levels/"+first.ID.String(), map[string]any{"name": "Inicio", "min_xp": 0}).Expect(t, http.StatusOK)
	s.admin.Do(http.MethodDelete, "/api/gamification/levels/"+first.ID.String(), nil).Expect(t, http.StatusConflict, "level_locked")
	s.admin.Do(http.MethodPatch, "/api/gamification/levels/"+second.ID.String(), map[string]any{"name": "Dos", "min_xp": 0}).Expect(t, http.StatusConflict, "level_conflict")
	s.admin.Do(http.MethodDelete, "/api/gamification/levels/"+second.ID.String(), nil).Expect(t, http.StatusNoContent)
	s.member.Do(http.MethodDelete, "/api/gamification/levels/"+created.ID.String(), nil).Expect(t, http.StatusForbidden)
	if got := points(t, s.member); got.Level.Number != 2 || got.Level.Name != "Aprendiz" {
		t.Fatalf("level = %+v after delete (RF-35)", got)
	}
	if got := s.env.AuditActions(second.ID.String()); len(got) != 1 || got[0] != "level.deleted" {
		t.Fatalf("audit = %v (RF-36)", got)
	}
}

func TestTeamRanking(t *testing.T) {
	s := newWorkSetup(t)
	// BI has the lead and the viewer directly, and Mario through BI Reportes (RF-43).
	ana := s.env.CreateUser(UserSpec{Name: "Ana", Email: "ana@x.com", Roles: []string{"member"}, Teams: []uuid.UUID{s.bi}})
	gone := s.env.CreateUser(UserSpec{Name: "Baja", Email: "baja@x.com", Roles: []string{"member"}, Teams: []uuid.UUID{s.bi}})
	s.exec(t, "INSERT INTO team_members (team_id, user_id, source) VALUES ($1, $2, 'manual')", s.bi, s.leadID)
	give := func(user uuid.UUID, xp int, at string) {
		s.exec(t, "ALTER TABLE point_ledger DISABLE TRIGGER USER")
		s.exec(t, "INSERT INTO point_ledger (user_id, xp, coins, reason, created_at) VALUES ($1, $2, 0, 'adjust', $3::timestamptz)", user, xp, at)
		s.exec(t, "ALTER TABLE point_ledger ENABLE TRIGGER USER")
	}
	give(s.memberID, 500, "2026-02-10T12:00:00Z") // a previous month
	give(s.memberID, 10, "2026-03-09T12:00:00Z")  // Monday of the week
	give(ana, 40, "2026-03-02T12:00:00Z")         // this month, last week
	give(ana, -5, "2026-03-10T12:00:00Z")         // a reopening counts (RF-49)
	give(gone, 900, "2026-03-10T12:00:00Z")
	s.exec(t, "UPDATE users SET status = 'inactive' WHERE id = $1", gone)

	type rankingDTO struct {
		Period string  `json:"period"`
		From   *string `json:"from"`
		To     *string `json:"to"`
		Items  []struct {
			Position int `json:"position"`
			User     struct {
				Name string `json:"name"`
			} `json:"user"`
			Level levelDTO `json:"level"`
			XP    int64    `json:"xp"`
		} `json:"items"`
	}
	rank := func(c *Client, query string) rankingDTO {
		var out rankingDTO
		c.Get("/api/gamification/teams/"+s.bi.String()+"/ranking?today=2026-03-11"+query).Expect(t, http.StatusOK).JSON(t, &out)
		return out
	}
	names := func(r rankingDTO) []string {
		var out []string
		for _, i := range r.Items {
			out = append(out, i.User.Name)
		}
		return out
	}
	eq := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}

	month := rank(s.lead, "")
	if month.Period != "month" || *month.From != "2026-03-01" || *month.To != "2026-03-31" || !eq(names(month), []string{"Ana", "Mario", "Laura", "viewer@x.com"}) ||
		month.Items[0].XP != 35 || month.Items[1].Level.Number != 3 || month.Items[2].Position != 3 {
		t.Fatalf("month = %+v (RF-44 to RF-49)", month)
	}
	week := rank(s.lead, "&period=week")
	if *week.From != "2026-03-09" || *week.To != "2026-03-15" || !eq(names(week), []string{"Mario", "Laura", "viewer@x.com", "Ana"}) {
		t.Fatalf("week = %+v (RF-47, RF-49)", week)
	}
	all := rank(s.lead, "&period=all")
	if all.From != nil || !eq(names(all), []string{"Mario", "Ana", "Laura", "viewer@x.com"}) || all.Items[0].XP != 510 {
		t.Fatalf("all = %+v (RF-47)", all)
	}
	// Nobody with points: everybody with 0, by name (RF-50).
	empty := func() rankingDTO {
		var out rankingDTO
		s.lead.Get("/api/gamification/teams/"+s.bi.String()+"/ranking?period=week&today=2026-01-07").Expect(t, http.StatusOK).JSON(t, &out)
		return out
	}()
	if !eq(names(empty), []string{"Ana", "Laura", "Mario", "viewer@x.com"}) || empty.Items[0].XP != 0 {
		t.Fatalf("empty = %+v (RF-50)", empty)
	}

	// Direct members only, or the manager (RF-52).
	s.member.Get("/api/gamification/teams/" + s.bi.String() + "/ranking").Expect(t, http.StatusForbidden)
	s.member.Get("/api/gamification/teams/" + s.rep.String() + "/ranking").Expect(t, http.StatusOK)
	s.admin.Get("/api/gamification/teams/" + s.other.String() + "/ranking").Expect(t, http.StatusOK)
	s.lead.Get("/api/gamification/teams/" + s.bi.String() + "/ranking?period=year").Expect(t, http.StatusBadRequest)
	s.lead.Get("/api/gamification/teams/" + uuid.NewString() + "/ranking").Expect(t, http.StatusNotFound)
}
