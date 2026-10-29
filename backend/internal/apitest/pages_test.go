package apitest

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type savedPageDTO struct {
	Page        json.RawMessage `json:"page"`
	PageVersion int32           `json:"page_version"`
	Version     int32           `json:"version"`
}

// ownerPage is the page part of a work item, milestone or project.
type ownerPage struct {
	Description json.RawMessage `json:"description"`
	PageVersion int32           `json:"page_version"`
	Version     int32           `json:"version"`
}

func (s *workSetup) readPage(t *testing.T, owner string) ownerPage {
	t.Helper()
	var o ownerPage
	s.admin.Get("/api/"+owner).Expect(t, http.StatusOK).JSON(t, &o)
	return o
}

func (s *workSetup) savePage(t *testing.T, c *Client, owner string, version int32, doc string, status int, code ...string) savedPageDTO {
	t.Helper()
	var p savedPageDTO
	res := c.Do(http.MethodPut, "/api/"+owner+"/page", map[string]any{"page": json.RawMessage(doc), "page_version": version}).
		Expect(t, status, code...)
	if status == http.StatusOK {
		res.JSON(t, &p)
	}
	return p
}

func paragraph(text string) string {
	return `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"` + text + `"}]}]}`
}

func withImage(id uuid.UUID) string {
	return `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"captura"}]},` +
		`{"type":"image","attrs":{"fileId":"` + id.String() + `","src":"/api/files/` + id.String() + `"}}]}`
}

// Autosave compares only the page version (RF-34, RF-59).
func TestSavePageAndConflicts(t *testing.T) {
	s := newWorkSetup(t)
	wi := s.item(t, s.member, s.workProject(t), map[string]any{})
	owner := "work-items/" + wi.ID.String()

	saved := s.savePage(t, s.member, owner, 1, paragraph("uno"), http.StatusOK)
	if saved.PageVersion != 2 || saved.Version != wi.Version+1 || !strings.Contains(string(saved.Page), "uno") {
		t.Fatalf("saved = %+v", saved)
	}
	// Someone else saved meanwhile: no overwrite, and the current version comes back.
	s.savePage(t, s.lead, owner, 1, paragraph("pisado"), http.StatusConflict, "page_conflict")
	if got := s.readPage(t, owner); !strings.Contains(string(got.Description), "uno") {
		t.Fatalf("page after conflict = %s", got.Description)
	}

	// A status change in between is no conflict, and keeps the page (RF-59).
	current := s.get(t, wi.ID)
	s.patch(s.member, current, map[string]any{"title": "Otro título"}).Expect(t, http.StatusOK)
	if got := s.readPage(t, owner); got.PageVersion != 2 || !strings.Contains(string(got.Description), "uno") {
		t.Fatalf("PATCH without description = %+v", got)
	}
	s.savePage(t, s.member, owner, 2, paragraph("dos"), http.StatusOK)

	// The PATCH of the previous frontend still saves the description and moves the page version.
	current = s.get(t, wi.ID)
	s.patch(s.member, current, map[string]any{"description": json.RawMessage(paragraph("tres"))}).Expect(t, http.StatusOK)
	if got := s.readPage(t, owner); got.PageVersion != 4 || !strings.Contains(string(got.Description), "tres") {
		t.Fatalf("PATCH with description = %+v", got)
	}
	s.savePage(t, s.member, owner, 3, paragraph("tarde"), http.StatusConflict, "page_conflict")

	// Who may save: the work item's writers (spec 004).
	s.savePage(t, s.viewer, owner, 4, paragraph("x"), http.StatusForbidden)
	s.savePage(t, s.outsider, owner, 4, paragraph("x"), http.StatusForbidden)
	s.savePage(t, s.member, "work-items/"+uuid.NewString(), 1, paragraph("x"), http.StatusNotFound)
	s.savePage(t, s.member, owner, 4, `{"type":"doc","content":[{"type":"iframe"}]}`, http.StatusBadRequest, "invalid_rich_text")

	// Only that it changed is audited, never the content (RF-47).
	var after string
	if err := s.env.Store.Pool.QueryRow(context.Background(),
		"SELECT after::text FROM audit_log WHERE action = 'work_item.page_updated' AND target_id = $1 ORDER BY id DESC LIMIT 1",
		wi.ID.String()).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != `{"description_changed": true}` {
		t.Fatalf("audit after = %s", after)
	}
}

// Saving marks the files the page no longer shows, and an undo brings them back (RF-49, RF-50).
func TestPageFilesAreMarked(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	wi := s.item(t, s.member, project, map[string]any{})
	owner := "work-items/" + wi.ID.String()
	img := s.upload(t, s.member, owner, "captura.png", pngData, http.StatusCreated)

	s.savePage(t, s.member, owner, 1, withImage(img.ID), http.StatusOK)
	s.member.Get("/api/files/"+img.ID.String()).Expect(t, http.StatusOK)
	s.savePage(t, s.member, owner, 2, paragraph("sin imagen"), http.StatusOK)
	s.member.Get("/api/files/"+img.ID.String()).Expect(t, http.StatusNotFound)
	s.savePage(t, s.member, owner, 3, withImage(img.ID), http.StatusOK)
	s.member.Get("/api/files/"+img.ID.String()).Expect(t, http.StatusOK)

	// Nothing is removed from the storage (RF-52).
	var rows int
	if err := s.env.Store.Pool.QueryRow(context.Background(), "SELECT count(*) FROM page_file_data WHERE file_id = $1", img.ID).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("content rows = %d, %v", rows, err)
	}

	// The file of another work item can't be shown through this page (ING-4).
	other := s.item(t, s.member, project, map[string]any{})
	theirs := s.upload(t, s.member, "work-items/"+other.ID.String(), "suya.png", pngData, http.StatusCreated)
	s.savePage(t, s.member, owner, 4, withImage(theirs.ID), http.StatusBadRequest, "invalid_rich_text")
}

// Milestones and projects have pages too; descriptions of spec 004 stay valid (RF-43, RF-44).
func TestPagesOfMilestonesAndProjects(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	m := s.milestone(t, project, map[string]any{"name": "Beta", "due_date": "2026-12-01", "description": json.RawMessage(paragraph("de la 004"))})
	owner := "milestones/" + m.ID.String()
	if got := s.readPage(t, owner); got.PageVersion != 1 || !strings.Contains(string(got.Description), "de la 004") {
		t.Fatalf("milestone = %+v", got)
	}
	task := `{"type":"doc","content":[{"type":"taskList","content":[{"type":"taskItem","attrs":{"checked":true},"content":[{"type":"paragraph","content":[{"type":"text","text":"hecho"}]}]}]}]}`
	s.savePage(t, s.lead, owner, 1, task, http.StatusOK)
	s.savePage(t, s.member, owner, 2, task, http.StatusForbidden)

	img := s.upload(t, s.lead, "projects/"+project.String(), "a.png", pngData, http.StatusCreated)
	saved := s.savePage(t, s.lead, "projects/"+project.String(), 1, withImage(img.ID), http.StatusOK)
	if saved.PageVersion != 2 {
		t.Fatalf("project page = %+v", saved)
	}
	s.savePage(t, s.member, "projects/"+project.String(), 2, paragraph("x"), http.StatusForbidden)
}
