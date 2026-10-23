package apitest

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// TestWorkAudit checks that the spec 004 changes are audited and that
// descriptions only appear as a flag (RF-134, RF-135).
func TestWorkAudit(t *testing.T) {
	s := newWorkSetup(t)
	secret := "texto-confidencial-de-la-descripcion"
	doc := map[string]any{"type": "doc", "content": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": secret}}}}}

	p := s.project(t, s.lead, map[string]any{"identifier": "AUD", "name": "Auditado", "description": doc})
	s.patchProject(s.lead, p, map[string]any{"description": nil}).Expect(t, http.StatusOK)
	wi := s.item(t, s.member, p.ID, map[string]any{"description": doc})
	s.patch(s.member, wi, map[string]any{"title": "Otro", "description": doc}).Expect(t, http.StatusOK)
	m := s.milestone(t, p.ID, map[string]any{"name": "M", "description": doc})
	s.link(t, s.lead, m.ID, wi.ID).Expect(t, http.StatusNoContent)
	o := s.objective(t, s.lead, map[string]any{"scope": "equipo", "scope_team_id": s.bi, "description": doc})

	for target, want := range map[string][]string{
		p.ID.String():  {"project.created", "project.updated"},
		wi.ID.String(): {"work_item.created", "work_item.updated"},
		m.ID.String():  {"milestone.created", "milestone.work_item_linked"},
		o.ID.String():  {"objective.created"},
	} {
		got := s.env.AuditActions(target)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("audit of %s = %v, want %v", target, got, want)
		}
	}

	var leaked int
	if err := s.env.Store.Pool.QueryRow(context.Background(),
		"SELECT count(*) FROM audit_log WHERE before::text LIKE $1 OR after::text LIKE $1", "%"+secret+"%").Scan(&leaked); err != nil {
		t.Fatal(err)
	}
	if leaked != 0 {
		t.Fatalf("%d audit entries contain the description text (RF-135)", leaked)
	}
	var flags []bool
	rows, err := s.env.Store.Pool.Query(context.Background(),
		"SELECT (after->>'description_changed')::boolean FROM audit_log WHERE action IN ('project.updated', 'work_item.updated') ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var f bool
		_ = rows.Scan(&f)
		flags = append(flags, f)
	}
	if len(flags) != 2 || !flags[0] || flags[1] {
		t.Fatalf("description_changed = %v, want [true false]", flags)
	}
}
