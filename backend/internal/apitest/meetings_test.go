package apitest

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type meetingDTO struct {
	ID          uuid.UUID       `json:"id"`
	Title       string          `json:"title"`
	StartsAt    string          `json:"starts_at"`
	Location    string          `json:"location"`
	Description json.RawMessage `json:"description"`
	PageVersion int32           `json:"page_version"`
	Version     int32           `json:"version"`
	TemplateID  *uuid.UUID      `json:"template_id"`
	Tasks       []struct {
		ID    uuid.UUID `json:"id"`
		Key   string    `json:"key"`
		Title string    `json:"title"`
	} `json:"tasks"`
	Can struct {
		Write   bool `json:"write"`
		Comment bool `json:"comment"`
	} `json:"can"`
}

type meetingPageDTO struct {
	Items []struct {
		ID    uuid.UUID `json:"id"`
		Title string    `json:"title"`
		Tasks int       `json:"tasks"`
	} `json:"items"`
	Total int `json:"total"`
}

type meetingTemplateDTO struct {
	ID      uuid.UUID       `json:"id"`
	Scope   string          `json:"scope"`
	Name    string          `json:"name"`
	Title   string          `json:"title"`
	Page    json.RawMessage `json:"page"`
	Retired bool            `json:"retired"`
}

func (s *workSetup) meeting(t *testing.T, c *Client, project uuid.UUID, body map[string]any) meetingDTO {
	t.Helper()
	var m meetingDTO
	c.Post("/api/projects/"+project.String()+"/meetings", body).Expect(t, http.StatusCreated).JSON(t, &m)
	return m
}

func (s *workSetup) meetings(t *testing.T, project uuid.UUID, when string) meetingPageDTO {
	t.Helper()
	var out meetingPageDTO
	s.viewer.Get("/api/projects/"+project.String()+"/meetings?when="+when).Expect(t, http.StatusOK).JSON(t, &out)
	return out
}

func TestMeetings(t *testing.T) {
	s := newWorkSetup(t)
	project := s.project(t, s.lead, map[string]any{"identifier": "REU", "name": "Reuniones"})
	path := "/api/projects/" + project.ID.String() + "/meetings"

	// Whoever writes work items creates them; the rest reads (RF-8, RF-9).
	s.viewer.Post(path, map[string]any{"title": "x", "starts_at": "2030-01-10T10:00:00Z"}).Expect(t, http.StatusForbidden)
	s.member.Post(path, map[string]any{"title": "x"}).Expect(t, http.StatusBadRequest)                                            // RF-4
	s.member.Post(path, map[string]any{"title": " ", "starts_at": "2030-01-10T10:00:00Z"}).Expect(t, http.StatusBadRequest)       // RF-3
	s.member.Post(path, map[string]any{"title": "x", "starts_at": "2030-01-10T10:00:00Z", "location": strings.Repeat("a", 501)}). // RF-5
																	Expect(t, http.StatusBadRequest)
	weekly := s.meeting(t, s.member, project.ID, map[string]any{"title": "Seguimiento", "starts_at": "2030-01-10T10:00:00-06:00", "location": "https://meet.example.com/x"})
	if !weekly.Can.Write || weekly.Location != "https://meet.example.com/x" {
		t.Fatalf("meeting = %+v", weekly)
	}
	s.meeting(t, s.member, project.ID, map[string]any{"title": "Arranque", "starts_at": "2020-05-01T09:00:00Z"})
	s.meeting(t, s.member, project.ID, map[string]any{"title": "Revisión", "starts_at": "2030-01-05T09:00:00Z"})

	// Upcoming nearest first, past latest first (RF-38).
	if up := s.meetings(t, project.ID, "upcoming"); up.Total != 2 || up.Items[0].Title != "Revisión" || up.Items[1].Title != "Seguimiento" {
		t.Fatalf("upcoming = %+v", up)
	}
	if past := s.meetings(t, project.ID, "past"); past.Total != 1 || past.Items[0].Title != "Arranque" {
		t.Fatalf("past = %+v", past)
	}

	// Edited with its version (RF-15).
	mpath := "/api/meetings/" + weekly.ID.String()
	var edited meetingDTO
	s.member.Do(http.MethodPatch, mpath, map[string]any{"version": weekly.Version, "title": "Seguimiento semanal", "starts_at": weekly.StartsAt, "location": "Sala 2"}).
		Expect(t, http.StatusOK).JSON(t, &edited)
	if edited.Title != "Seguimiento semanal" || edited.Location != "Sala 2" {
		t.Fatalf("edited = %+v", edited)
	}
	s.member.Do(http.MethodPatch, mpath, map[string]any{"version": weekly.Version, "title": "x", "starts_at": weekly.StartsAt}).
		Expect(t, http.StatusConflict, "version_conflict")

	// The notes are a page: a new mention tells the person (RF-16, RF-19).
	s.savePage(t, s.member, "meetings/"+weekly.ID.String(), edited.PageVersion, string(mentionDoc("Para", s.outsiderID)), http.StatusOK)
	s.env.App.Events.Poll(t.Context())
	if n := notificationsOf(t, s.outsider, true); n.Unread != 1 || n.Items[0].Subject.Type != "meeting" || n.Items[0].Subject.Title != "Seguimiento semanal" {
		t.Fatalf("mention = %+v", n)
	}
	// And a meeting is mentioned with # (RF-18).
	var found struct {
		Meetings []struct {
			ID uuid.UUID `json:"id"`
		} `json:"meetings"`
	}
	s.viewer.Get("/api/mentions?q=semanal").Expect(t, http.StatusOK).JSON(t, &found)
	if len(found.Meetings) != 1 || found.Meetings[0].ID != weekly.ID {
		t.Fatalf("search = %+v", found)
	}

	// A work item from the meeting, linked both ways (RF-24, RF-26 to RF-28).
	act := s.env.CatalogID("work_item_types", "Actividad")
	task := s.item(t, s.member, project.ID, map[string]any{"type_id": act, "title": "Enviar el acta", "meeting_id": weekly.ID})
	var wi struct {
		Meeting *struct {
			ID   uuid.UUID `json:"id"`
			Name string    `json:"name"`
		} `json:"meeting"`
	}
	s.member.Get("/api/work-items/"+task.ID.String()).Expect(t, http.StatusOK).JSON(t, &wi)
	if wi.Meeting == nil || wi.Meeting.ID != weekly.ID || wi.Meeting.Name != "Seguimiento semanal" {
		t.Fatalf("work item meeting = %+v", wi.Meeting)
	}
	var got meetingDTO
	s.viewer.Get(mpath).Expect(t, http.StatusOK).JSON(t, &got)
	if len(got.Tasks) != 1 || got.Tasks[0].Title != "Enviar el acta" || got.Can.Write {
		t.Fatalf("tasks = %+v, can = %+v", got.Tasks, got.Can)
	}
	if up := s.meetings(t, project.ID, "upcoming"); up.Items[1].Tasks != 1 { // RF-39
		t.Fatalf("count = %+v", up.Items)
	}
	other := s.project(t, s.lead, map[string]any{"identifier": "OTR", "name": "Otro"})
	s.member.Post("/api/projects/"+other.ID.String()+"/work-items", map[string]any{"type_id": act, "title": "x", "meeting_id": weekly.ID}).
		Expect(t, http.StatusBadRequest)

	// Comments as in work items, and replies reach the thread (RF-20 to RF-22).
	var c commentsResult
	s.member.Post(mpath+"/comments", text("¿Quién lleva el acta?")).Expect(t, http.StatusCreated).JSON(t, &c)
	root := c.Comments.Threads[0].ID
	s.lead.Post(mpath+"/comments", map[string]any{"body": json.RawMessage(paragraph("Yo")), "thread_id": root}).Expect(t, http.StatusCreated)
	s.viewer.Post(mpath+"/comments", text("x")).Expect(t, http.StatusForbidden)
	s.env.App.Events.Poll(t.Context())
	if n := notificationsOf(t, s.member, true); n.Unread != 1 || n.Items[0].Last.Kind != "reply" || n.Items[0].Subject.Type != "meeting" {
		t.Fatalf("reply = %+v", n)
	}
	s.lead.Post("/api/comments/"+root.String()+"/resolve", nil).Expect(t, http.StatusOK)

	// Deleted: its comments go, its work items stay without the link (RF-12, RF-13).
	s.viewer.Do(http.MethodDelete, mpath, nil).Expect(t, http.StatusForbidden)
	s.member.Do(http.MethodDelete, mpath, nil).Expect(t, http.StatusNoContent)
	s.member.Get(mpath).Expect(t, http.StatusNotFound)
	s.member.Get("/api/work-items/"+task.ID.String()).Expect(t, http.StatusOK).JSON(t, &wi)
	if wi.Meeting != nil {
		t.Fatalf("still linked = %+v", wi.Meeting)
	}
	var left int
	if err := s.env.Store.Pool.QueryRow(t.Context(), "SELECT count(*) FROM work_item_comments WHERE meeting_id = $1", weekly.ID).Scan(&left); err != nil || left != 0 {
		t.Fatalf("comments left = %d %v", left, err)
	}
	if a := s.env.AuditActions(weekly.ID.String()); len(a) < 3 || a[0] != "meeting.created" || a[len(a)-1] != "meeting.deleted" {
		t.Fatalf("audit = %v", a)
	}

	// A closed project leaves them read only (RF-10).
	review := s.meetings(t, project.ID, "upcoming").Items[0]
	s.lead.Post("/api/projects/"+project.ID.String()+"/close", map[string]any{"version": s.readProjectVersion(t, project.ID)}).Expect(t, http.StatusOK)
	s.member.Do(http.MethodPatch, "/api/meetings/"+review.ID.String(), map[string]any{"version": 1, "title": "x", "starts_at": "2030-01-05T09:00:00Z"}).
		Expect(t, http.StatusConflict, "project_closed")
}

func (s *workSetup) readProjectVersion(t *testing.T, id uuid.UUID) int32 {
	t.Helper()
	var p projectDTO
	s.admin.Get("/api/projects/"+id.String()).Expect(t, http.StatusOK).JSON(t, &p)
	return p.Version
}

func TestMeetingTemplates(t *testing.T) {
	s := newWorkSetup(t)
	project := s.project(t, s.lead, map[string]any{"identifier": "PLT", "name": "Plantillas"})
	agenda := json.RawMessage(`{"type":"doc","content":[{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Orden del día"}]}]}`)

	// Global ones for the catalog administrator, a project's for its managers (RF-29, RF-30).
	s.member.Post("/api/meeting-templates", map[string]any{"name": "Seguimiento"}).Expect(t, http.StatusForbidden)
	var global meetingTemplateDTO
	s.admin.Post("/api/meeting-templates", map[string]any{"name": "Seguimiento", "title": "Seguimiento semanal", "location": "Sala 1", "page": agenda}).
		Expect(t, http.StatusCreated).JSON(t, &global)
	s.admin.Post("/api/meeting-templates", map[string]any{"name": " seguimiento "}).Expect(t, http.StatusConflict, "meeting_template_name_taken") // RF-33
	s.member.Post("/api/meeting-templates", map[string]any{"name": "Del equipo", "project_id": project.ID}).Expect(t, http.StatusForbidden)
	var own meetingTemplateDTO
	s.lead.Post("/api/meeting-templates", map[string]any{"name": "Seguimiento", "project_id": project.ID, "title": "Del proyecto"}).
		Expect(t, http.StatusCreated).JSON(t, &own)
	// No images nor files in a template (RF-32).
	image := `{"type":"doc","content":[{"type":"image","attrs":{"fileId":"` + uuid.NewString() + `"}}]}`
	s.admin.Post("/api/meeting-templates", map[string]any{"name": "Con imagen", "page": json.RawMessage(image)}).Expect(t, http.StatusBadRequest)

	// The choice: the global ones, then the project's (RF-34).
	var choose struct {
		Items []meetingTemplateDTO `json:"items"`
	}
	s.member.Get("/api/meeting-templates?project_id="+project.ID.String()).Expect(t, http.StatusOK).JSON(t, &choose)
	if len(choose.Items) != 2 || choose.Items[0].Scope != "global" || choose.Items[1].Scope != "project" {
		t.Fatalf("choose = %+v", choose.Items)
	}

	// From a template: its title, place and page (RF-35).
	m := s.meeting(t, s.member, project.ID, map[string]any{"starts_at": "2030-02-01T10:00:00Z", "template_id": global.ID,
		"description": json.RawMessage(paragraph("Notas del día"))})
	if m.Title != "Seguimiento semanal" || m.Location != "Sala 1" || !strings.Contains(string(m.Description), "Orden del día") ||
		!strings.Contains(string(m.Description), "Notas del día") || m.TemplateID == nil || *m.TemplateID != global.ID {
		t.Fatalf("from template = %+v %s", m, m.Description)
	}

	// Editing or retiring it leaves the meetings as they are (RF-36).
	s.admin.Do(http.MethodPatch, "/api/meeting-templates/"+global.ID.String(), map[string]any{"name": "Seguimiento", "title": "Otro título"}).Expect(t, http.StatusOK)
	var again meetingDTO
	s.member.Get("/api/meetings/"+m.ID.String()).Expect(t, http.StatusOK).JSON(t, &again)
	if again.Title != "Seguimiento semanal" {
		t.Fatalf("changed = %+v", again)
	}
	s.admin.Post("/api/meeting-templates/"+global.ID.String()+"/retire", nil).Expect(t, http.StatusOK)
	s.member.Get("/api/meeting-templates?project_id="+project.ID.String()).Expect(t, http.StatusOK).JSON(t, &choose)
	if len(choose.Items) != 1 || choose.Items[0].ID != own.ID {
		t.Fatalf("after retiring = %+v", choose.Items)
	}
	s.member.Post("/api/projects/"+project.ID.String()+"/meetings", map[string]any{"starts_at": "2030-02-01T10:00:00Z", "template_id": global.ID}).
		Expect(t, http.StatusBadRequest)

	// The global ones retire, the project's are deleted.
	s.admin.Do(http.MethodDelete, "/api/meeting-templates/"+global.ID.String(), nil).Expect(t, http.StatusBadRequest)
	s.lead.Do(http.MethodDelete, "/api/meeting-templates/"+own.ID.String(), nil).Expect(t, http.StatusNoContent)
	if a := s.env.AuditActions(global.ID.String()); len(a) != 3 || a[2] != "meeting_template.retired" { // RF-37
		t.Fatalf("audit = %v", a)
	}
}

func TestMCPMeetings(t *testing.T) {
	s := newWorkSetup(t)
	env := s.env
	enableMCP(t, s.admin, true)
	project := s.project(t, s.lead, map[string]any{"identifier": "MCR", "name": "Con IA"})
	conn := connect(t, s.lead, "Claude")
	var tpl meetingTemplateDTO
	s.admin.Post("/api/meeting-templates", map[string]any{"name": "Comité", "title": "Comité de seguimiento",
		"page": json.RawMessage(`{"type":"doc","content":[{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Orden del día"}]}]}`)}).
		Expect(t, http.StatusCreated).JSON(t, &tpl)

	// The time zone is required (RF-52).
	msg, failed := env.tool(t, conn, "create_meeting", map[string]any{"project": "MCR", "title": "x", "starts_at": "2030-03-01T10:00:00"})
	if !failed || !strings.Contains(msg, "zona horaria") {
		t.Fatalf("no zone = %v %s", failed, msg)
	}
	// With template and notes, the notes after the template's page (RF-46, RF-47).
	m := env.toolOK(t, conn, "create_meeting", map[string]any{"project": "MCR", "starts_at": "2030-03-01T10:00:00+01:00",
		"template_id": tpl.ID.String(), "notes": "Acuerdos:\n\n- revisar el plan"})
	notes, _ := m["description"].(string)
	if m["title"] != "Comité de seguimiento" || !strings.HasPrefix(notes, "## Orden del día") || !strings.Contains(notes, "- revisar el plan") ||
		m["url"] != Origin+"/meetings/"+m["id"].(string) {
		t.Fatalf("created = %v", m)
	}
	mid := m["id"].(string)

	// A work item from it, mentioned at the end of its notes (RF-51).
	act := env.CatalogID("work_item_types", "Actividad")
	task := env.toolOK(t, conn, "create_meeting_task", map[string]any{"meeting": mid, "type_id": act.String(), "title": "Revisar el plan"})
	got := env.toolOK(t, conn, "get_meeting", map[string]any{"meeting": mid})
	if tasks, _ := got["tasks"].([]any); len(tasks) != 1 || !strings.Contains(got["description"].(string), "@"+task["key"].(string)) {
		t.Fatalf("meeting = %v", got)
	}
	env.toolOK(t, conn, "add_meeting_comment", map[string]any{"meeting": mid, "text": "Hecho"})
	env.toolOK(t, conn, "update_meeting", map[string]any{"meeting": mid, "location": "Sala 3"})
	list := env.toolOK(t, conn, "list_meetings", map[string]any{"project": project.ID.String()})
	if items, _ := list["items"].([]any); len(items) != 1 || items[0].(map[string]any)["location"] != "Sala 3" {
		t.Fatalf("list = %v", list)
	}
	if a := env.AuditActions(mid); len(a) == 0 || a[0] != "meeting.created" {
		t.Fatalf("audit = %v", a)
	}
}
