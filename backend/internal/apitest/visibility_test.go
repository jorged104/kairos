package apitest

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// Spec 023: who sees each project. In newWorkSetup the lead leads BI, the
// member is in BI Reportes, the viewer in BI and the outsider in Otro.

func TestProjectVisibility(t *testing.T) {
	s := newWorkSetup(t)
	ventas := s.env.CreateTeam("BI Ventas", &s.bi)
	bi := s.project(t, s.lead, map[string]any{"identifier": "BIP", "name": "De BI"})
	sibling := s.project(t, s.admin, map[string]any{"identifier": "VEN", "name": "De Ventas", "team_id": ventas})
	other := s.project(t, s.admin, map[string]any{"identifier": "AJE", "name": "Ajeno", "team_id": s.other})
	director := s.env.As(s.env.CreateUser(UserSpec{Email: "dir@x.com", Roles: []string{"director"}}))

	see := func(c *Client, p projectDTO, want int) {
		t.Helper()
		c.Get("/api/projects/" + p.ID.String()).Expect(t, want)
	}
	// Ancestor team yes, sibling no (Conceptos 3); the viewer of BI sees its subteams.
	see(s.member, bi, http.StatusOK)
	see(s.member, sibling, http.StatusNotFound)
	see(s.member, other, http.StatusNotFound)
	see(s.viewer, sibling, http.StatusOK)
	// The lead sees their scope (Conceptos 4); Dirección sees all, and manages nothing (RF-7, RF-9).
	see(s.lead, sibling, http.StatusOK)
	see(s.lead, other, http.StatusNotFound)
	see(director, other, http.StatusOK)
	s.patchProject(director, other, map[string]any{"name": "No"}).Expect(t, http.StatusForbidden, "forbidden")

	// A role given on purpose shows it (Conceptos 2).
	s.addMember(s.admin, other.ID, s.memberID, "observer").Expect(t, http.StatusOK)
	see(s.member, other, http.StatusOK)

	// The # search finds only what one sees (RF-2).
	var found struct {
		Projects []struct {
			Label string `json:"label"`
		} `json:"projects"`
	}
	s.lead.Get("/api/mentions?q=Ajeno").Expect(t, http.StatusOK).JSON(t, &found)
	if len(found.Projects) != 0 {
		t.Fatalf("lead finds = %+v", found.Projects)
	}
	s.admin.Get("/api/mentions?q=Ajeno").Expect(t, http.StatusOK).JSON(t, &found)
	if len(found.Projects) != 1 {
		t.Fatalf("admin finds = %+v", found.Projects)
	}

	// A new project of one's team shows in the list at once (RF-4).
	var list struct {
		Items []projectDTO `json:"items"`
	}
	s.member.Get("/api/projects").Expect(t, http.StatusOK)
	s.project(t, s.lead, map[string]any{"identifier": "NUE", "name": "Nuevo"})
	s.member.Get("/api/projects?q=nuevo").Expect(t, http.StatusOK).JSON(t, &list)
	if len(list.Items) != 1 {
		t.Fatalf("new project = %+v", list.Items)
	}
}

func TestAssignedInUnseenProject(t *testing.T) {
	s := newWorkSetup(t)
	other := s.project(t, s.admin, map[string]any{"identifier": "AJE", "name": "Ajeno", "team_id": s.other})
	mine := s.item(t, s.admin, other.ID, map[string]any{"title": "Para Mario", "assignee_id": s.memberID, "due_date": "2026-12-01"})
	notMine := s.item(t, s.admin, other.ID, map[string]any{"title": "De otro"})

	// The assignee sees the project, read only (Conceptos 5, RF-15).
	s.member.Get("/api/projects/" + other.ID.String()).Expect(t, http.StatusOK)
	var got struct {
		workItemDTO
		Can struct {
			Write    bool `json:"write"`
			Progress bool `json:"progress"`
		} `json:"can"`
	}
	s.member.Get("/api/work-items/"+mine.ID.String()).Expect(t, http.StatusOK).JSON(t, &got)
	if got.Can.Write || !got.Can.Progress || len(got.AllowedStatusIDs) == 0 {
		t.Fatalf("own work item = %+v", got.Can)
	}
	// Status and progress change; anything else stays.
	s.patch(s.member, mine, map[string]any{"done_ratio": 40, "title": "Cambiado"}).Expect(t, http.StatusOK)
	if after := s.get(t, mine.ID); after.DoneRatio != 40 || after.Title != "Para Mario" {
		t.Fatalf("after = %+v", after)
	}
	s.patch(s.member, notMine, map[string]any{"done_ratio": 10}).Expect(t, http.StatusForbidden, "forbidden")
	s.member.Post("/api/projects/"+other.ID.String()+"/work-items", map[string]any{"type_id": s.env.CatalogID("work_item_types", "Actividad"), "title": "x"}).
		Expect(t, http.StatusForbidden, "forbidden")

	// In their panel it moves (RF-15); in the lead's view of it, it is hidden (RF-26).
	var mineDash dashboardDTO
	s.member.Get("/api/me/dashboard").Expect(t, http.StatusOK).JSON(t, &mineDash)
	if len(mineDash.Tasks) != 1 || !mineDash.Tasks[0].CanMove || mineDash.Tasks[0].Title != "Para Mario" {
		t.Fatalf("own panel = %+v", mineDash.Tasks)
	}
	var theirs struct {
		Tasks []struct {
			ID     uuid.UUID `json:"id"`
			Title  string    `json:"title"`
			Hidden bool      `json:"hidden"`
		} `json:"tasks"`
	}
	s.lead.Get("/api/users/"+s.memberID.String()+"/dashboard").Expect(t, http.StatusOK).JSON(t, &theirs)
	if len(theirs.Tasks) != 1 || !theirs.Tasks[0].Hidden || theirs.Tasks[0].Title != "" || theirs.Tasks[0].ID != uuid.Nil {
		t.Fatalf("lead sees = %+v", theirs.Tasks)
	}
	var team struct {
		People []struct {
			User struct {
				ID uuid.UUID `json:"id"`
			} `json:"user"`
			Bars []struct {
				Title  string `json:"title"`
				Hidden bool   `json:"hidden"`
			} `json:"bars"`
		} `json:"people"`
	}
	s.lead.Get("/api/me/team").Expect(t, http.StatusOK).JSON(t, &team)
	for _, p := range team.People {
		if p.User.ID == s.memberID && (len(p.Bars) != 1 || !p.Bars[0].Hidden || p.Bars[0].Title != "") {
			t.Fatalf("lead's schedule = %+v", p.Bars)
		}
	}

	// Without the work item, the project is gone at once (RF-16).
	s.patch(s.admin, s.get(t, mine.ID), map[string]any{"assignee_id": s.outsiderID}).Expect(t, http.StatusOK)
	s.member.Get("/api/projects/" + other.ID.String()).Expect(t, http.StatusNotFound)
}

func TestHiddenLinks(t *testing.T) {
	s := newWorkSetup(t)
	bi := s.workProject(t)
	other := s.project(t, s.admin, map[string]any{"identifier": "AJE", "name": "Ajeno", "team_id": s.other})
	a := s.item(t, s.member, bi, map[string]any{"title": "Mía"})
	x := s.item(t, s.admin, other.ID, map[string]any{"title": "Secreta"})
	s.admin.Post("/api/work-items/"+x.ID.String()+"/dependencies", map[string]any{"to_id": a.ID, "type": "blocks"}).Expect(t, http.StatusCreated)

	// A dependency on a project one does not see shows without key nor title (RF-27).
	var wi struct {
		Dependencies []struct {
			Other struct {
				Key    string `json:"key"`
				Title  string `json:"title"`
				Hidden bool   `json:"hidden"`
			} `json:"other"`
		} `json:"dependencies"`
	}
	s.member.Get("/api/work-items/"+a.ID.String()).Expect(t, http.StatusOK).JSON(t, &wi)
	if len(wi.Dependencies) != 1 || !wi.Dependencies[0].Other.Hidden || wi.Dependencies[0].Other.Title != "" {
		t.Fatalf("dependencies = %+v", wi.Dependencies)
	}

	// An objective counts its projects and shows only the visible ones (RF-29).
	o := s.objective(t, s.admin, map[string]any{"title": "O", "scope": "organizacion", "owner_id": s.adminID,
		"period_start": "2026-01-01", "period_end": "2026-12-31"})
	for _, p := range []uuid.UUID{bi, other.ID} {
		s.admin.Post("/api/objectives/"+o.ID.String()+"/projects", map[string]any{"id": p}).Expect(t, http.StatusOK)
	}
	var ob struct {
		Projects []struct {
			Name string `json:"name"`
		} `json:"projects"`
		Hidden int `json:"hidden_projects"`
	}
	s.member.Get("/api/objectives/"+o.ID.String()).Expect(t, http.StatusOK).JSON(t, &ob)
	if len(ob.Projects) != 1 || ob.Hidden != 1 {
		t.Fatalf("objective = %+v", ob)
	}
}

func TestDirectorRole(t *testing.T) {
	s := newWorkSetup(t)
	dirID := s.env.CreateUser(UserSpec{Name: "Dora", Email: "dora@x.com", Roles: []string{"member"}, Teams: []uuid.UUID{s.rep}})
	path := "/api/users/" + dirID.String() + "/roles"
	// Only the administrator gives it (RF-11); then a lead cannot act on them (RF-10).
	s.lead.Post(path, map[string]any{"role": "director"}).Expect(t, http.StatusForbidden, "forbidden")
	s.admin.Post(path, map[string]any{"role": "director"}).Expect(t, http.StatusOK)
	s.lead.Post(path, map[string]any{"role": "viewer"}).Expect(t, http.StatusForbidden, "forbidden")

	var me struct {
		Permissions []string `json:"permissions"`
	}
	s.env.As(dirID).Get("/api/me").Expect(t, http.StatusOK).JSON(t, &me)
	if p := strings.Join(me.Permissions, " "); !strings.Contains(p, "oversight:view") || !strings.Contains(p, "projects:read:all") || strings.Contains(p, "users:create") {
		t.Fatalf("director permissions = %v", me.Permissions)
	}
}

func TestOversightToolsInMCP(t *testing.T) {
	s := newWorkSetup(t)
	enableMCP(t, s.admin, true)
	tools := func(c *Client) string {
		t.Helper()
		var listed struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		}
		r := s.env.rpc(t, connect(t, c, "Claude"), "tools/list", nil)
		if err := json.Unmarshal(r.Result, &listed); err != nil {
			t.Fatal(err)
		}
		names := []string{}
		for _, tl := range listed.Tools {
			names = append(names, tl.Name)
		}
		return " " + strings.Join(names, " ") + " "
	}
	// Portafolio and Seguimiento only for who has them (RF-17a).
	if got := tools(s.member); strings.Contains(got, " portfolio ") || strings.Contains(got, " tracking ") {
		t.Fatalf("member tools = %s", got)
	}
	if got := tools(s.lead); !strings.Contains(got, " portfolio ") || !strings.Contains(got, " tracking ") {
		t.Fatalf("lead tools = %s", got)
	}
}
