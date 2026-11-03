package apitest

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

type memberDTO struct {
	User struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	} `json:"user"`
	Role     string `json:"role"`
	Owner    bool   `json:"owner"`
	InTeam   bool   `json:"in_team"`
	Explicit bool   `json:"explicit"`
	CanWork  bool   `json:"can_work"`
	Capacity *int   `json:"capacity"`
	Load     struct {
		Project    int  `json:"project"`
		Total      int  `json:"total"`
		Items      int  `json:"items"`
		Overloaded bool `json:"overloaded"`
	} `json:"load"`
}

func (s *workSetup) members(t *testing.T, c *Client, project uuid.UUID) map[uuid.UUID]memberDTO {
	t.Helper()
	var out struct {
		Items []memberDTO `json:"items"`
	}
	c.Get("/api/projects/"+project.String()+"/members").Expect(t, http.StatusOK).JSON(t, &out)
	m := map[uuid.UUID]memberDTO{}
	for _, it := range out.Items {
		m[it.User.ID] = it
	}
	return m
}

func (s *workSetup) setRole(c *Client, project, user uuid.UUID, role string) *Response {
	return c.Do(http.MethodPatch, "/api/projects/"+project.String()+"/members/"+user.String(), map[string]any{"role": role})
}

func (s *workSetup) addMember(c *Client, project, user uuid.UUID, role string) *Response {
	return c.Post("/api/projects/"+project.String()+"/members", map[string]any{"user_id": user, "role": role})
}

func TestProjectRoles(t *testing.T) {
	s := newWorkSetup(t)
	p := s.project(t, s.lead, map[string]any{"identifier": "KAI", "name": "Kairos"})
	act := s.env.CatalogID("work_item_types", "Actividad")
	newItem := func(c *Client) *Response {
		return c.Post("/api/projects/"+p.ID.String()+"/work-items", map[string]any{"type_id": act, "title": "x"})
	}

	// The owner is PM and the team's participants are members (RF-34 to RF-36).
	team := s.members(t, s.lead, p.ID)
	if team[s.leadID].Role != "pm" || !team[s.leadID].Owner || team[s.memberID].Role != "member" || !team[s.memberID].InTeam {
		t.Fatalf("team = %+v", team)
	}
	if _, ok := team[s.outsiderID]; ok {
		t.Fatal("an outsider is not a member")
	}
	// Only who manages the team sees and changes it (RF-4, RF-33).
	s.member.Get("/api/projects/"+p.ID.String()+"/members").Expect(t, http.StatusForbidden)
	s.addMember(s.member, p.ID, s.outsiderID, "member").Expect(t, http.StatusForbidden)
	newItem(s.outsider).Expect(t, http.StatusForbidden)

	// A PM manages the project even with the global role member (RF-39).
	s.addMember(s.lead, p.ID, s.outsiderID, "pm").Expect(t, http.StatusOK)
	var proj struct {
		Can struct {
			ManageTeam bool `json:"manage_team"`
		} `json:"can"`
		MyRole string `json:"my_role"`
	}
	s.outsider.Get("/api/projects/"+p.ID.String()).Expect(t, http.StatusOK).JSON(t, &proj)
	if !proj.Can.ManageTeam || proj.MyRole != "pm" {
		t.Fatalf("PM sees %+v", proj)
	}
	s.patchProject(s.outsider, s.getProject(t, p.ID), map[string]any{"name": "Kairos 2"}).Expect(t, http.StatusOK)
	newItem(s.outsider).Expect(t, http.StatusCreated)

	// An observer only reads and comments, even from the team (RF-42).
	byMember := s.item(t, s.member, p.ID, map[string]any{"title": "de Mario"})
	s.setRole(s.outsider, p.ID, s.memberID, "observer").Expect(t, http.StatusOK)
	newItem(s.member).Expect(t, http.StatusForbidden)
	s.patch(s.member, s.get(t, byMember.ID), map[string]any{"title": "no"}).Expect(t, http.StatusForbidden)
	s.member.Post("/api/work-items/"+byMember.ID.String()+"/comments", map[string]any{"body": json.RawMessage(paragraph("ok"))}).
		Expect(t, http.StatusCreated)

	// A tech lead deletes anyone's work items but doesn't manage (RF-40, RF-80).
	tl := s.env.CreateUser(UserSpec{Name: "Tere", Email: "tl@x.com", Roles: []string{"member"}, Teams: []uuid.UUID{s.other}})
	tech := s.env.As(tl)
	s.addMember(s.outsider, p.ID, tl, "tech_lead").Expect(t, http.StatusOK)
	tech.Do(http.MethodDelete, "/api/work-items/"+byMember.ID.String(), nil).Expect(t, http.StatusNoContent)
	tech.Get("/api/projects/"+p.ID.String()+"/members").Expect(t, http.StatusForbidden)
	s.patchProject(tech, s.getProject(t, p.ID), map[string]any{"name": "No"}).Expect(t, http.StatusForbidden)

	// A role never takes away a global grant (RF-43).
	s.addMember(s.outsider, p.ID, s.adminID, "observer").Expect(t, http.StatusOK)
	newItem(s.admin).Expect(t, http.StatusCreated)

	// Rules of the team (RF-35, RF-37, RF-38).
	s.setRole(s.outsider, p.ID, s.leadID, "member").Expect(t, http.StatusConflict, "owner_is_pm")
	viewerID := s.env.CreateUser(UserSpec{Email: "v9@x.com", Roles: []string{"viewer"}})
	s.addMember(s.outsider, p.ID, viewerID, "member").Expect(t, http.StatusBadRequest, "invalid_member")
	s.addMember(s.outsider, p.ID, viewerID, "observer").Expect(t, http.StatusOK)
	s.outsider.Do(http.MethodDelete, "/api/projects/"+p.ID.String()+"/members/"+s.memberID.String(), nil).
		Expect(t, http.StatusConflict, "member_from_team")
	s.addMember(s.outsider, p.ID, s.outsiderID, "bogus").Expect(t, http.StatusBadRequest, "validation_failed")

	// Taking the PM out takes the grant away at once (RF-44).
	s.lead.Do(http.MethodDelete, "/api/projects/"+p.ID.String()+"/members/"+s.outsiderID.String(), nil).Expect(t, http.StatusOK)
	newItem(s.outsider).Expect(t, http.StatusForbidden)

	// Every change is audited on the project (RF-45).
	n := 0
	for _, a := range s.env.AuditActions(p.ID.String()) {
		if len(a) > 15 && a[:15] == "project.member_" {
			n++
		}
	}
	if n != 6 {
		t.Fatalf("member audit entries = %d, want 6", n)
	}

	// A closed project's team is read-only.
	s.lead.Post("/api/projects/"+p.ID.String()+"/close", map[string]any{"version": s.getProject(t, p.ID).Version}).Expect(t, http.StatusOK)
	s.addMember(s.lead, p.ID, s.outsiderID, "member").Expect(t, http.StatusConflict, "project_closed")
}

func TestCapacityLoadAndSuggestions(t *testing.T) {
	s := newWorkSetup(t)
	p := s.project(t, s.lead, map[string]any{"identifier": "KAI", "name": "Kairos"})
	other := s.project(t, s.lead, map[string]any{"identifier": "OTR", "name": "Otro"})
	capacity := func(c *Client, user uuid.UUID, v any) *Response {
		return c.Do(http.MethodPut, "/api/users/"+user.String()+"/capacity", map[string]any{"capacity": v})
	}

	// Administrators and the person's leader set it, 0 to 1000 (RF-46, RF-47).
	capacity(s.member, s.memberID, 10).Expect(t, http.StatusForbidden)
	capacity(s.lead, s.memberID, 1001).Expect(t, http.StatusBadRequest, "validation_failed")
	capacity(s.lead, s.memberID, 5).Expect(t, http.StatusOK)
	capacity(s.admin, s.leadID, 20).Expect(t, http.StatusOK)
	var u struct {
		Capacity *int `json:"capacity"`
	}
	s.admin.Get("/api/users/"+s.memberID.String()).Expect(t, http.StatusOK).JSON(t, &u)
	if u.Capacity == nil || *u.Capacity != 5 {
		t.Fatalf("capacity = %v, want 5", u.Capacity)
	}

	// Load in this project and in all of them; over capacity is overloaded (RF-48, RF-49).
	s.item(t, s.member, p.ID, map[string]any{"weight": 4, "assignee_ids": []uuid.UUID{s.memberID}})
	s.item(t, s.member, other.ID, map[string]any{"weight": 3, "assignee_ids": []uuid.UUID{s.memberID}})
	closed := s.item(t, s.member, p.ID, map[string]any{"weight": 9, "assignee_ids": []uuid.UUID{s.memberID}})
	s.patch(s.admin, s.get(t, closed.ID), map[string]any{"status_id": s.env.CatalogID("statuses", "Cerrado"),
		"assignee_ids": []uuid.UUID{s.memberID}}).Expect(t, http.StatusOK)
	team := s.members(t, s.lead, p.ID)
	if l := team[s.memberID].Load; l.Project != 4 || l.Total != 7 || l.Items != 1 || !l.Overloaded {
		t.Fatalf("member load = %+v", l)
	}
	if l := team[s.leadID].Load; l.Total != 0 || l.Overloaded {
		t.Fatalf("lead load = %+v", l)
	}

	// Suggestions: whoever shares a label first, then the least loaded (RF-52).
	label := s.label(t, "Frontend")
	tl := s.env.CreateUser(UserSpec{Name: "Tere", Email: "tl@x.com", Roles: []string{"member"}, Teams: []uuid.UUID{s.rep}})
	s.item(t, s.member, p.ID, map[string]any{"weight": 1, "assignee_ids": []uuid.UUID{tl}, "label_ids": []uuid.UUID{label}})
	free := s.item(t, s.member, p.ID, map[string]any{"title": "Libre", "weight": 2, "label_ids": []uuid.UUID{label}})
	plain := s.item(t, s.member, p.ID, map[string]any{"title": "Sin etiqueta", "weight": 1})
	var sg struct {
		Items []struct {
			ID     uuid.UUID `json:"id"`
			People []struct {
				ID uuid.UUID `json:"id"`
			} `json:"people"`
		} `json:"items"`
	}
	s.lead.Get("/api/projects/"+p.ID.String()+"/assignment-suggestions").Expect(t, http.StatusOK).JSON(t, &sg)
	got := map[uuid.UUID][]uuid.UUID{}
	for _, it := range sg.Items {
		for _, pp := range it.People {
			got[it.ID] = append(got[it.ID], pp.ID)
		}
	}
	if len(got[free.ID]) != 2 || got[free.ID][0] != tl {
		t.Fatalf("suggestions for a Frontend item = %v, want Tere first", got[free.ID])
	}
	// Without a shared label: the lead (0 of 20) before the member (overloaded); no capacity last.
	if len(got[plain.ID]) != 2 || got[plain.ID][0] != s.leadID || got[plain.ID][1] != s.memberID {
		t.Fatalf("suggestions without label = %v, want lead then member", got[plain.ID])
	}
	// Observers are never suggested.
	s.setRole(s.lead, p.ID, tl, "observer").Expect(t, http.StatusOK)
	s.lead.Get("/api/projects/"+p.ID.String()+"/assignment-suggestions").Expect(t, http.StatusOK).JSON(t, &sg)
	for _, it := range sg.Items {
		for _, pp := range it.People {
			if pp.ID == tl {
				t.Fatal("an observer was suggested")
			}
		}
	}
	s.member.Get("/api/projects/"+p.ID.String()+"/assignment-suggestions").Expect(t, http.StatusForbidden)
}

func (s *workSetup) getProject(t *testing.T, id uuid.UUID) projectDTO {
	t.Helper()
	var p projectDTO
	s.admin.Get("/api/projects/"+id.String()).Expect(t, http.StatusOK).JSON(t, &p)
	return p
}
