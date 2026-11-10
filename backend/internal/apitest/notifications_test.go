package apitest

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

type notificationDTO struct {
	ID      uuid.UUID `json:"id"`
	Subject struct {
		Type  string    `json:"type"`
		ID    uuid.UUID `json:"id"`
		Key   string    `json:"key"`
		Title string    `json:"title"`
	} `json:"subject"`
	Actors []struct {
		Name string `json:"name"`
	} `json:"actors"`
	ActorsTotal int  `json:"actors_total"`
	Events      int  `json:"events"`
	Mention     bool `json:"mention"`
	Last        struct {
		Kind      string     `json:"kind"`
		CommentID *uuid.UUID `json:"comment_id"`
		Excerpt   string     `json:"excerpt"`
		From      string     `json:"from"`
		To        string     `json:"to"`
	} `json:"last"`
	ReadAt *string `json:"read_at"`
}

type notificationsDTO struct {
	Items  []notificationDTO `json:"items"`
	Total  int               `json:"total"`
	Unread int               `json:"unread"`
}

func notificationsOf(t *testing.T, c *Client, unread bool) notificationsDTO {
	t.Helper()
	path := "/api/notifications"
	if unread {
		path += "?unread=true"
	}
	var out notificationsDTO
	c.Get(path).Expect(t, http.StatusOK).JSON(t, &out)
	return out
}

func mentionDoc(text string, user uuid.UUID) json.RawMessage {
	return json.RawMessage(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"` + text + ` "},` +
		`{"type":"mention","attrs":{"kind":"user","id":"` + user.String() + `"}}]}]}`)
}

func TestNotificationsOfComments(t *testing.T) {
	s := newWorkSetup(t)
	env := s.env
	project := s.project(t, s.lead, map[string]any{"identifier": "NOT", "name": "Avisos"})
	wi := s.item(t, s.lead, project.ID, map[string]any{"title": "Revisar informe", "assignee_ids": []uuid.UUID{s.memberID}})
	path := "/api/work-items/" + wi.ID.String()
	poll := func() { env.App.Events.Poll(t.Context()) }

	// The assignee follows the work item (RF-1, RF-2); its author does not.
	var f struct {
		Following bool `json:"following"`
	}
	s.member.Get(path+"/follow").Expect(t, http.StatusOK).JSON(t, &f)
	if !f.Following {
		t.Fatal("the assignee does not follow")
	}
	s.lead.Get(path+"/follow").Expect(t, http.StatusOK).JSON(t, &f)
	if f.Following {
		t.Fatal("the author follows")
	}

	// It reaches the open tab at once (RF-33).
	conn, _, err := dialWS(t, env, &s.memberID, Origin)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// A root comment: the followers, never its author (RF-7, RF-15, RF-24).
	root := s.comment(t, s.lead, wi.ID, text("Faltan las cifras de octubre"), http.StatusCreated).Comments.Threads[0]
	poll()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var msg struct {
		Type         string          `json:"type"`
		Unread       int             `json:"unread"`
		Notification notificationDTO `json:"notification"`
	}
	if err := conn.ReadJSON(&msg); err != nil || msg.Type != "notifications" || msg.Unread != 1 || msg.Notification.Last.Kind != "comment" {
		t.Fatalf("realtime = %+v %v", msg, err)
	}
	n := notificationsOf(t, s.member, false)
	if n.Unread != 1 || len(n.Items) != 1 {
		t.Fatalf("member = %+v", n)
	}
	got := n.Items[0]
	if got.Subject.Key != "NOT-1" || got.Subject.Title != "Revisar informe" || got.Actors[0].Name != "Laura" ||
		got.Last.Excerpt != "Faltan las cifras de octubre" || got.Last.CommentID == nil || *got.Last.CommentID != root.ID {
		t.Fatalf("notification = %+v", got)
	}
	if notificationsOf(t, s.lead, false).Total != 0 {
		t.Fatal("the author was told of their own comment")
	}

	// A reply joins the unread one (RF-21), and reaches the thread's participants (RF-9).
	s.addMember(s.lead, project.ID, s.outsiderID, "observer").Expect(t, http.StatusOK)
	s.comment(t, s.outsider, wi.ID, map[string]any{"body": json.RawMessage(paragraph("Las añado")), "thread_id": root.ID}, http.StatusCreated)
	poll()
	n = notificationsOf(t, s.member, false)
	if len(n.Items) != 1 || n.Items[0].Events != 2 || n.Items[0].ActorsTotal != 2 || n.Items[0].Actors[0].Name != "Otto" || n.Items[0].Last.Kind != "reply" {
		t.Fatalf("grouped = %+v", n.Items)
	}
	if l := notificationsOf(t, s.lead, true); l.Unread != 1 || l.Items[0].Last.Kind != "reply" {
		t.Fatalf("participant = %+v", l)
	}

	// Read, the next event starts another one (RF-25, RF-40).
	s.member.Do(http.MethodPut, "/api/notifications/"+n.Items[0].ID.String()+"/read", map[string]any{"read": true}).Expect(t, http.StatusNoContent)
	s.comment(t, s.lead, wi.ID, text("Gracias"), http.StatusCreated)
	poll()
	if n = notificationsOf(t, s.member, false); n.Total != 2 || n.Unread != 1 || n.Items[0].ReadAt != nil || n.Items[1].ReadAt == nil {
		t.Fatalf("after reading = %+v", n)
	}

	// Stopped following: no comments, but mentions still arrive (RF-4, RF-10, RF-12, RF-17).
	s.member.Post("/api/notifications/read-all", nil).Expect(t, http.StatusNoContent)
	s.member.Do(http.MethodPut, path+"/follow", map[string]any{"following": false}).Expect(t, http.StatusOK)
	s.comment(t, s.lead, wi.ID, text("Otra cosa"), http.StatusCreated)
	poll()
	if n = notificationsOf(t, s.member, true); n.Unread != 0 {
		t.Fatalf("unfollowed = %+v", n)
	}
	s.comment(t, s.lead, wi.ID, map[string]any{"body": mentionDoc("Mira esto", s.memberID)}, http.StatusCreated)
	poll()
	if n = notificationsOf(t, s.member, true); n.Unread != 1 || !n.Items[0].Mention {
		t.Fatalf("mention = %+v", n)
	}

	// Assigned again, follows again (RF-5); a status change reaches the followers (RF-13).
	s.member.Post("/api/notifications/read-all", nil).Expect(t, http.StatusNoContent)
	s.moveItem(t, wi.ID, s.env.CatalogID("statuses", "Nuevo"), map[string]any{"assignee_ids": []uuid.UUID{}})
	s.moveItem(t, wi.ID, s.env.CatalogID("statuses", "En progreso"), map[string]any{"assignee_ids": []uuid.UUID{s.memberID}})
	if n = notificationsOf(t, s.member, true); n.Unread != 1 || n.Items[0].Last.Kind != "status" || n.Items[0].Last.From != "Nuevo" || n.Items[0].Last.To != "En progreso" {
		t.Fatalf("status = %+v", n.Items)
	}

	// Resolving a thread reaches its participants (RF-14).
	s.lead.Post("/api/notifications/read-all", nil).Expect(t, http.StatusNoContent)
	s.member.Post("/api/comments/"+root.ID.String()+"/resolve", nil).Expect(t, http.StatusOK)
	poll()
	if l := notificationsOf(t, s.lead, true); l.Unread != 1 || l.Items[0].Last.Kind != "thread_resolved" {
		t.Fatalf("resolved = %+v", l.Items)
	}

	// A repeated delivery counts once.
	before := notificationsOf(t, s.lead, true).Items[0].Events
	s.exec(t, "UPDATE event_consumers SET last_event_id = last_event_id - 1 WHERE name = 'notifications'")
	poll()
	if after := notificationsOf(t, s.lead, true).Items[0].Events; after != before {
		t.Fatalf("events %d → %d", before, after)
	}

	// Marked unread by hand (RF-42); gone with its work item (RF-45).
	s.member.Do(http.MethodPut, "/api/notifications/"+n.Items[0].ID.String()+"/read", map[string]any{"read": false}).Expect(t, http.StatusNoContent)
	s.lead.Do(http.MethodPut, "/api/notifications/"+n.Items[0].ID.String()+"/read", map[string]any{"read": true}).Expect(t, http.StatusNotFound)
	var count struct {
		Unread int `json:"unread"`
	}
	s.member.Get("/api/notifications/unread").Expect(t, http.StatusOK).JSON(t, &count)
	if count.Unread == 0 {
		t.Fatal("not unread")
	}
	s.exec(t, "DELETE FROM work_item_comments WHERE work_item_id = $1", wi.ID)
	s.admin.Do(http.MethodDelete, path, nil).Expect(t, http.StatusNoContent)
	if n = notificationsOf(t, s.member, false); n.Total != 0 || n.Unread != 0 {
		t.Fatalf("deleted = %+v", n)
	}
}

func TestNotificationsOfPageMentions(t *testing.T) {
	s := newWorkSetup(t)
	project := s.project(t, s.lead, map[string]any{"identifier": "MEN", "name": "Menciones"})
	wi := s.item(t, s.lead, project.ID, map[string]any{"title": "Plan"})
	owner := "work-items/" + wi.ID.String()

	// A new mention in a description, even without following (RF-11, RF-12).
	page := s.readPage(t, owner)
	saved := s.savePage(t, s.lead, owner, page.PageVersion, string(mentionDoc("Revisa", s.memberID)), http.StatusOK)
	s.env.App.Events.Poll(t.Context())
	n := notificationsOf(t, s.member, true)
	if n.Unread != 1 || n.Items[0].Last.Kind != "mention" || !n.Items[0].Mention || n.Items[0].Subject.Key != "MEN-1" {
		t.Fatalf("mention = %+v", n)
	}
	// Saving again with the same mention tells nothing new.
	s.member.Post("/api/notifications/read-all", nil).Expect(t, http.StatusNoContent)
	s.savePage(t, s.lead, owner, saved.PageVersion, string(mentionDoc("Revisa ya", s.memberID)), http.StatusOK)
	s.env.App.Events.Poll(t.Context())
	if n = notificationsOf(t, s.member, true); n.Unread != 0 {
		t.Fatalf("again = %+v", n)
	}

	// A milestone's description too, by name (RF-11).
	m := s.milestone(t, project.ID, map[string]any{"name": "Entrega", "description": mentionDoc("Para", s.memberID)})
	s.env.App.Events.Poll(t.Context())
	if n = notificationsOf(t, s.member, true); n.Unread != 1 || n.Items[0].Subject.Type != "milestone" || n.Items[0].Subject.Title != "Entrega" || n.Items[0].Subject.ID != m.ID {
		t.Fatalf("milestone = %+v", n)
	}
}
