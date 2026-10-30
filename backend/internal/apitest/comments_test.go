package apitest

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type commentDTO struct {
	ID     uuid.UUID `json:"id"`
	Author struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	} `json:"author"`
	Body     json.RawMessage `json:"body"`
	EditedAt *string         `json:"edited_at"`
	Deleted  bool            `json:"deleted"`
	Can      struct {
		Edit   bool `json:"edit"`
		Delete bool `json:"delete"`
	} `json:"can"`
}

type threadDTO struct {
	commentDTO
	Quote         *string `json:"quote"`
	Anchored      bool    `json:"anchored"`
	AnchorDeleted bool    `json:"anchor_deleted"`
	ResolvedAt    *string `json:"resolved_at"`
	ResolvedBy    *struct {
		ID uuid.UUID `json:"id"`
	} `json:"resolved_by"`
	Replies []commentDTO `json:"replies"`
}

type commentsDTO struct {
	Threads    []threadDTO                `json:"threads"`
	Refs       map[string]json.RawMessage `json:"refs"`
	CanComment bool                       `json:"can_comment"`
}

type commentsResult struct {
	Comments commentsDTO   `json:"comments"`
	Page     *savedPageDTO `json:"page"`
}

func (s *workSetup) comments(t *testing.T, c *Client, wi uuid.UUID) commentsDTO {
	t.Helper()
	var out commentsDTO
	c.Get("/api/work-items/"+wi.String()+"/comments").Expect(t, http.StatusOK).JSON(t, &out)
	return out
}

func (s *workSetup) comment(t *testing.T, c *Client, wi uuid.UUID, body map[string]any, status int, code ...string) commentsResult {
	t.Helper()
	var out commentsResult
	res := c.Post("/api/work-items/"+wi.String()+"/comments", body).Expect(t, status, code...)
	if status == http.StatusCreated {
		res.JSON(t, &out)
	}
	return out
}

func text(s string) map[string]any { return map[string]any{"body": json.RawMessage(paragraph(s))} }

func TestCommentThreads(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	wi := s.item(t, s.member, project, map[string]any{}).ID

	// Everyone but viewers comments (RF-23 to RF-25); viewers read (RF-24).
	first := s.comment(t, s.member, wi, text("¿Usamos SSO?"), http.StatusCreated).Comments.Threads[0]
	s.comment(t, s.viewer, wi, text("yo"), http.StatusForbidden)
	if got := s.comments(t, s.viewer, wi); got.CanComment || len(got.Threads) != 1 || got.Threads[0].Can.Edit {
		t.Fatalf("viewer sees %+v", got)
	}
	// Someone of another team reads every project and comments too (spec 004 visibility).
	s.comment(t, s.outsider, wi, text("Desde fuera"), http.StatusCreated)

	// Empty and over 10 000 characters are rejected with their reason (RF-27).
	s.comment(t, s.member, wi, map[string]any{"body": nil}, http.StatusBadRequest, "comment_empty")
	s.comment(t, s.member, wi, text(strings.Repeat("a", 10_001)), http.StatusBadRequest, "comment_too_long")
	s.comment(t, s.member, wi, text(strings.Repeat("a", 10_000)), http.StatusCreated)

	// Replies, one level: answering a reply goes to the same thread (RF-28, RF-29).
	r := s.comment(t, s.lead, wi, map[string]any{"body": json.RawMessage(paragraph("Sí")), "thread_id": first.ID}, http.StatusCreated)
	reply := r.Comments.Threads[0].Replies[0]
	r = s.comment(t, s.member, wi, map[string]any{"body": json.RawMessage(paragraph("Vale")), "thread_id": reply.ID}, http.StatusCreated)
	if th := r.Comments.Threads; len(th) != 3 || th[0].ID != first.ID || len(th[0].Replies) != 2 || th[0].Replies[1].Author.ID != s.memberID {
		t.Fatalf("threads = %+v", th)
	}
	s.comment(t, s.member, wi, map[string]any{"body": json.RawMessage(paragraph("x")), "thread_id": uuid.New()}, http.StatusNotFound)

	// The author edits their own, marked as edited (RF-30, RF-31); nobody else does (RF-35).
	var res commentsResult
	s.member.Do(http.MethodPatch, "/api/comments/"+first.ID.String(), text("¿Usamos SAML?")).Expect(t, http.StatusOK).JSON(t, &res)
	if th := res.Comments.Threads[0]; th.EditedAt == nil || !strings.Contains(string(th.Body), "SAML") {
		t.Fatalf("edited = %+v", th)
	}
	s.lead.Do(http.MethodPatch, "/api/comments/"+first.ID.String(), text("mío")).Expect(t, http.StatusForbidden)

	// Resolving folds it, answering reopens it (RF-36 to RF-38).
	s.outsider.Post("/api/comments/"+first.ID.String()+"/resolve", nil).Expect(t, http.StatusOK).JSON(t, &res)
	if th := res.Comments.Threads[0]; th.ResolvedAt == nil || th.ResolvedBy == nil || th.ResolvedBy.ID != s.outsiderID {
		t.Fatalf("resolved = %+v", th)
	}
	s.member.Post("/api/comments/"+reply.ID.String()+"/resolve", nil).Expect(t, http.StatusBadRequest, "validation_failed")
	r = s.comment(t, s.lead, wi, map[string]any{"body": json.RawMessage(paragraph("Una más")), "thread_id": first.ID}, http.StatusCreated)
	if r.Comments.Threads[0].ResolvedAt != nil {
		t.Fatal("answering a resolved thread doesn't reopen it (RF-38)")
	}
	s.lead.Post("/api/comments/"+first.ID.String()+"/resolve", nil).Expect(t, http.StatusOK)
	s.lead.Post("/api/comments/"+first.ID.String()+"/reopen", nil).Expect(t, http.StatusOK).JSON(t, &res)
	if res.Comments.Threads[0].ResolvedAt != nil {
		t.Fatal("reopen kept it resolved (RF-37)")
	}

	// A root with replies stays as deleted; the replies stay (RF-33).
	s.outsider.Do(http.MethodDelete, "/api/comments/"+first.ID.String(), nil).Expect(t, http.StatusForbidden)
	s.member.Do(http.MethodDelete, "/api/comments/"+first.ID.String(), nil).Expect(t, http.StatusOK).JSON(t, &res)
	if th := res.Comments.Threads[0]; !th.Deleted || string(th.Body) != "null" || len(th.Replies) != 3 {
		t.Fatalf("deleted root = %+v", th)
	}
	s.member.Do(http.MethodPatch, "/api/comments/"+first.ID.String(), text("vuelvo")).Expect(t, http.StatusNotFound)
	// The project manager deletes other people's (RF-34); the last reply takes the thread.
	replies := append([]commentDTO(nil), res.Comments.Threads[0].Replies...)
	for _, c := range replies {
		s.lead.Do(http.MethodDelete, "/api/comments/"+c.ID.String(), nil).Expect(t, http.StatusOK).JSON(t, &res)
	}
	if len(res.Comments.Threads) != 2 || res.Comments.Threads[0].ID == first.ID {
		t.Fatalf("threads after deleting every reply = %+v", res.Comments.Threads)
	}
	// Without replies a comment just goes (RF-32).
	alone := res.Comments.Threads[0].ID
	s.outsider.Do(http.MethodDelete, "/api/comments/"+alone.String(), nil).Expect(t, http.StatusOK).JSON(t, &res)
	if len(res.Comments.Threads) != 1 {
		t.Fatalf("threads = %d, want 1", len(res.Comments.Threads))
	}

	// Mentions resolve like the pages' (RF-26).
	mention := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Mira "},` +
		`{"type":"mention","attrs":{"kind":"user","id":"` + s.leadID.String() + `"}}]}]}`
	r = s.comment(t, s.member, wi, map[string]any{"body": json.RawMessage(mention)}, http.StatusCreated)
	if _, ok := r.Comments.Refs[s.leadID.String()]; !ok {
		t.Fatalf("refs = %v", r.Comments.Refs)
	}

	// Audited without the text (RF-42).
	if got := strings.Join(s.env.AuditActions(first.ID.String()), ","); !strings.HasPrefix(got, "work_item.comment_created,work_item.comment_updated,work_item.comment_resolved") {
		t.Fatalf("audit = %s", got)
	}
}

func TestCommentsInClosedProjects(t *testing.T) {
	s := newWorkSetup(t)
	p := s.project(t, s.lead, map[string]any{"identifier": "KAI", "name": "Kairos"})
	wi := s.item(t, s.member, p.ID, map[string]any{}).ID
	root := s.comment(t, s.member, wi, text("antes"), http.StatusCreated).Comments.Threads[0].ID
	s.lead.Post("/api/projects/"+p.ID.String()+"/close", map[string]any{"version": p.Version}).Expect(t, http.StatusOK)

	// Read-only (RF-39).
	if got := s.comments(t, s.member, wi); got.CanComment || len(got.Threads) != 1 || got.Threads[0].Can.Edit {
		t.Fatalf("closed project comments = %+v", got)
	}
	s.comment(t, s.member, wi, text("después"), http.StatusConflict, "project_closed")
	s.member.Do(http.MethodPatch, "/api/comments/"+root.String(), text("x")).Expect(t, http.StatusConflict, "project_closed")
	s.member.Post("/api/comments/"+root.String()+"/resolve", nil).Expect(t, http.StatusConflict, "project_closed")
}

func TestCommentFiles(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	wi := s.item(t, s.member, project, map[string]any{}).ID
	other := s.item(t, s.member, project, map[string]any{}).ID

	// A file of the comments goes in a comment of that work item only (RF-26).
	var f fileDTO
	s.outsider.Upload("/api/work-items/"+wi.String()+"/comment-files", "captura.png", pngData).Expect(t, http.StatusCreated).JSON(t, &f)
	s.viewer.Upload("/api/work-items/"+wi.String()+"/comment-files", "captura.png", pngData).Expect(t, http.StatusForbidden)
	s.comment(t, s.outsider, wi, map[string]any{"body": json.RawMessage(withImage(f.ID))}, http.StatusCreated)
	s.comment(t, s.outsider, other, map[string]any{"body": json.RawMessage(withImage(f.ID))}, http.StatusBadRequest, "invalid_rich_text")
	// Nor in the page: the comments' files are not the page's.
	s.savePage(t, s.member, "work-items/"+wi.String(), 1, withImage(f.ID), http.StatusBadRequest, "invalid_rich_text")
	s.viewer.Get("/api/files/"+f.ID.String()).Expect(t, http.StatusOK)

	// Deleting the work item takes its comments and stops serving their files (RF-40).
	s.member.Do(http.MethodDelete, "/api/work-items/"+wi.String(), nil).Expect(t, http.StatusNoContent)
	s.admin.Get("/api/files/"+f.ID.String()).Expect(t, http.StatusNotFound)
	s.admin.Get("/api/work-items/"+wi.String()+"/comments").Expect(t, http.StatusNotFound)
}

// anchored is a page whose "mundo" carries the mark of thread id.
func anchored(id uuid.UUID, more ...uuid.UUID) string {
	marks := `{"type":"comment","attrs":{"threadId":"` + id.String() + `"}}`
	for _, m := range more {
		marks += `,{"type":"comment","attrs":{"threadId":"` + m.String() + `"}}`
	}
	return `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hola "},` +
		`{"type":"text","text":"mundo","marks":[` + marks + `]}]}]}`
}

func TestAnchoredComments(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)
	wi := s.item(t, s.member, project, map[string]any{}).ID
	owner := "work-items/" + wi.String()
	saved := s.savePage(t, s.member, owner, 1, paragraph("Hola mundo"), http.StatusOK)

	anchor := func(c *Client, id uuid.UUID, page string, version int32, status int, code ...string) commentsResult {
		t.Helper()
		return s.comment(t, c, wi, map[string]any{"body": json.RawMessage(paragraph("¿Qué mundo?")),
			"anchor": map[string]any{"thread_id": id, "quote": "mundo", "page": json.RawMessage(page), "page_version": version}}, status, code...)
	}

	// Someone who can't edit the work item anchors a comment (RF-45, RF-46).
	t1 := uuid.New()
	var seen workItemDTO
	s.outsider.Get("/api/work-items/"+wi.String()).Expect(t, http.StatusOK).JSON(t, &seen)
	if seen.Can.Write {
		t.Fatal("setup: the outsider can write")
	}
	res := anchor(s.outsider, t1, anchored(t1), saved.PageVersion, http.StatusCreated)
	if res.Page == nil || res.Page.PageVersion != saved.PageVersion+1 || !strings.Contains(string(res.Page.Page), t1.String()) {
		t.Fatalf("page after anchoring = %+v", res.Page)
	}
	th := res.Comments.Threads[0]
	if th.ID != t1 || !th.Anchored || th.Quote == nil || *th.Quote != "mundo" || th.AnchorDeleted {
		t.Fatalf("anchored thread = %+v", th)
	}
	version := res.Page.PageVersion

	// Anchoring can't change the page or drop other anchors (RF-46).
	t2 := uuid.New()
	changed := strings.Replace(anchored(t1, t2), "Hola", "Adiós", 1)
	anchor(s.outsider, t2, changed, version, http.StatusConflict, "anchor_page_changed")
	anchor(s.outsider, t2, anchored(t2), version, http.StatusConflict, "anchor_page_changed")
	anchor(s.outsider, t2, anchored(t1), version, http.StatusConflict, "anchor_page_changed")
	// Nor over an older version, nor with an id in use.
	anchor(s.outsider, t2, anchored(t1, t2), version-1, http.StatusConflict, "page_conflict")
	anchor(s.outsider, t1, anchored(t1), version, http.StatusBadRequest, "validation_failed")
	// Viewers don't anchor (RF-55).
	anchor(s.viewer, t2, anchored(t1, t2), version, http.StatusForbidden)
	// Two anchors on the same text (RF-54).
	res = anchor(s.member, t2, anchored(t1, t2), version, http.StatusCreated)
	version = res.Page.PageVersion

	// Deleting the commented text keeps the thread with its quote (RF-51).
	s.savePage(t, s.member, owner, version, paragraph("Hola"), http.StatusOK)
	got := s.comments(t, s.member, wi)
	if len(got.Threads) != 2 || !got.Threads[0].AnchorDeleted || *got.Threads[0].Quote != "mundo" {
		t.Fatalf("threads after deleting the text = %+v", got.Threads)
	}
}

// TestCommentsPerformance reads 200 comments in 50 threads against the limit
// of 300 ms (median of 5, spec 006 RNF).
func TestCommentsPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("prueba de rendimiento omitida con -short")
	}
	s := newWorkSetup(t)
	wi := s.item(t, s.member, s.workProject(t), map[string]any{}).ID
	mention := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"` + strings.Repeat("texto ", 50) + `"},` +
		`{"type":"mention","attrs":{"kind":"user","id":"` + s.leadID.String() + `"}}]}]}`
	for i := 0; i < 50; i++ {
		root := s.comment(t, s.member, wi, map[string]any{"body": json.RawMessage(mention)}, http.StatusCreated).Comments.Threads[i].ID
		for j := 0; j < 3; j++ {
			s.comment(t, s.lead, wi, map[string]any{"body": json.RawMessage(mention), "thread_id": root}, http.StatusCreated)
		}
	}
	times := make([]time.Duration, 0, 5)
	for i := 0; i < 5; i++ {
		start := time.Now()
		got := s.comments(t, s.member, wi)
		times = append(times, time.Since(start))
		if len(got.Threads) != 50 {
			t.Fatalf("threads = %d", len(got.Threads))
		}
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	t.Logf("200 comentarios en 50 hilos: mediana %v (límite 300ms)", times[2].Round(time.Millisecond))
	if times[2] > 300*time.Millisecond {
		t.Errorf("comentarios: mediana %v supera 300ms", times[2])
	}
}
