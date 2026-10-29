package apitest

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
)

var fileIDRe = regexp.MustCompile(`"fileId":"([0-9a-f-]{36})"`)

// fileOf returns the only file a page shows.
func fileOf(t *testing.T, page json.RawMessage) uuid.UUID {
	t.Helper()
	m := fileIDRe.FindAllStringSubmatch(string(page), -1)
	if len(m) != 1 {
		t.Fatalf("page %s shows %d files, want 1", page, len(m))
	}
	return uuid.MustParse(m[0][1])
}

// Deleting a work item or a milestone marks its files (RF-51).
func TestDeletingMarksFiles(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	wi := s.item(t, s.member, project, map[string]any{})
	f := s.upload(t, s.member, "work-items/"+wi.ID.String(), "a.png", pngData, http.StatusCreated)
	s.savePage(t, s.member, "work-items/"+wi.ID.String(), 1, withImage(f.ID), http.StatusOK)
	s.member.Do(http.MethodDelete, "/api/work-items/"+wi.ID.String(), nil).Expect(t, http.StatusNoContent)
	s.member.Get("/api/files/"+f.ID.String()).Expect(t, http.StatusNotFound)

	m := s.milestone(t, project, map[string]any{"name": "Beta", "due_date": "2026-12-01"})
	g := s.upload(t, s.lead, "milestones/"+m.ID.String(), "a.png", pngData, http.StatusCreated)
	s.lead.Do(http.MethodDelete, "/api/milestones/"+m.ID.String(), nil).Expect(t, http.StatusNoContent)
	s.lead.Get("/api/files/"+g.ID.String()).Expect(t, http.StatusNotFound)
}

// A project from a template gets its own copy of every page's files (RF-53).
func TestTemplateCopiesDuplicateFiles(t *testing.T) {
	t.Run("db", func(t *testing.T) { templateCopy(t, newWorkSetup(t)) })
	t.Run("s3", func(t *testing.T) { templateCopy(t, newWorkSetupOn(t, New(t, WithS3(t)))) })
}

func templateCopy(t *testing.T, s *workSetup) {
	act := s.env.CatalogID("work_item_types", "Actividad")
	tpl := s.project(t, s.admin, map[string]any{"identifier": "TPL", "name": "Plantilla", "work_item_type_ids": []uuid.UUID{act}})
	m := s.milestone(t, tpl.ID, map[string]any{"name": "Entrega", "due_date": "2026-01-31"})
	wi := s.item(t, s.admin, tpl.ID, map[string]any{"title": "Con captura"})
	originals := map[string]uuid.UUID{}
	for _, owner := range []string{"projects/" + tpl.ID.String(), "milestones/" + m.ID.String(), "work-items/" + wi.ID.String()} {
		f := s.upload(t, s.admin, owner, "a.png", pngData, http.StatusCreated)
		s.savePage(t, s.admin, owner, 1, withImage(f.ID), http.StatusOK)
		originals[strings.SplitN(owner, "/", 2)[0]] = f.ID
	}
	s.admin.Post("/api/projects/"+tpl.ID.String()+"/template", nil).Expect(t, http.StatusOK)

	var copied projectDTO
	s.lead.Post("/api/projects/from-template", map[string]any{"template_id": tpl.ID, "identifier": "CPY", "name": "Copia",
		"team_id": s.bi, "include_content": true}).Expect(t, http.StatusCreated).JSON(t, &copied)

	ctx := context.Background()
	var milestoneID, itemID uuid.UUID
	if err := s.env.Store.Pool.QueryRow(ctx, "SELECT id FROM milestones WHERE project_id = $1", copied.ID).Scan(&milestoneID); err != nil {
		t.Fatal(err)
	}
	if err := s.env.Store.Pool.QueryRow(ctx, "SELECT id FROM work_items WHERE project_id = $1", copied.ID).Scan(&itemID); err != nil {
		t.Fatal(err)
	}
	copies := map[string]string{
		"projects":   "projects/" + copied.ID.String(),
		"milestones": "milestones/" + milestoneID.String(),
		"work-items": "work-items/" + itemID.String(),
	}
	for kind, owner := range copies {
		page := s.readPage(t, owner)
		f := fileOf(t, page.Description)
		if f == originals[kind] {
			t.Fatalf("%s: the copy shows the template's file", kind)
		}
		res := s.lead.Get("/api/files/"+f.String()).Expect(t, http.StatusOK)
		if string(res.Body) != string(pngData) {
			t.Fatalf("%s: copied content differs", kind)
		}
		// Dropping the copy's file leaves the template's untouched.
		s.savePage(t, s.lead, owner, page.PageVersion, paragraph("sin imagen"), http.StatusOK)
		s.lead.Get("/api/files/"+f.String()).Expect(t, http.StatusNotFound)
		s.admin.Get("/api/files/"+originals[kind].String()).Expect(t, http.StatusOK)
	}
}
