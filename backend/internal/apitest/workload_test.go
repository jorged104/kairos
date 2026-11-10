package apitest

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// Spec 021: cronograma por persona.

type workloadRowDTO struct {
	User struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	} `json:"user"`
	FreeFrom *string `json:"free_from"`
	Open     int     `json:"open"`
	Weight   int64   `json:"weight"`
	Undated  int     `json:"undated"`
	Bars     []struct {
		Key     string `json:"key"`
		Title   string `json:"title"`
		Project struct {
			Identifier string `json:"identifier"`
		} `json:"project"`
		Start   string `json:"start"`
		End     string `json:"end"`
		Overdue bool   `json:"overdue"`
	} `json:"bars"`
	// Only in Mi equipo.
	Teams []struct {
		Name string `json:"name"`
	} `json:"teams"`
	Capacity   *int32 `json:"capacity"`
	Overloaded bool   `json:"overloaded"`
}

func TestTeamSchedule(t *testing.T) {
	s := newWorkSetup(t)
	p := s.project(t, s.lead, map[string]any{"identifier": "ALF", "name": "Alfa"})
	s.item(t, s.lead, p.ID, map[string]any{"title": "A", "assignee_ids": []uuid.UUID{s.memberID}, "start_date": "2026-03-02", "due_date": "2026-03-06"})
	s.item(t, s.lead, p.ID, map[string]any{"title": "B", "assignee_ids": []uuid.UUID{s.memberID}, "due_date": "2026-03-20"})
	s.item(t, s.lead, p.ID, map[string]any{"title": "C", "assignee_ids": []uuid.UUID{s.memberID}})
	s.item(t, s.lead, p.ID, map[string]any{"title": "D", "assignee_ids": []uuid.UUID{s.leadID}, "due_date": "2026-03-01"})
	// Mario's work in another project counts only in Mi equipo.
	o := s.project(t, s.admin, map[string]any{"identifier": "OTR", "name": "Otro", "team_id": s.other, "owner_id": s.outsiderID})
	s.item(t, s.admin, o.ID, map[string]any{"title": "E", "assignee_ids": []uuid.UUID{s.memberID}, "due_date": "2026-04-01"})

	// The project: Laura is busy until today for her overdue item, Mario until
	// the 20th; the viewer, who can only read, is not there (RF-1 to RF-7, RF-17, RF-18).
	var res struct {
		Items []workloadRowDTO `json:"items"`
	}
	s.lead.Get("/api/projects/"+p.ID.String()+"/team-schedule?today=2026-03-10").Expect(t, http.StatusOK).JSON(t, &res)
	if len(res.Items) != 2 {
		t.Fatalf("filas = %+v", res.Items)
	}
	laura, mario := res.Items[0], res.Items[1]
	if laura.User.ID != s.leadID || laura.FreeFrom == nil || *laura.FreeFrom != "2026-03-11" || len(laura.Bars) != 1 || !laura.Bars[0].Overdue {
		t.Fatalf("Laura = %+v", laura)
	}
	if mario.User.ID != s.memberID || mario.FreeFrom == nil || *mario.FreeFrom != "2026-03-21" || mario.Open != 3 || mario.Undated != 1 || len(mario.Bars) != 2 {
		t.Fatalf("Mario = %+v", mario)
	}
	if b := mario.Bars[1]; b.Title != "B" || b.Start != "2026-03-20" || b.End != "2026-03-20" || b.Overdue || b.Key != "ALF-2" {
		t.Fatalf("barra sin inicio = %+v", b)
	}
	s.member.Get("/api/projects/"+p.ID.String()+"/team-schedule").Expect(t, http.StatusForbidden)

	// Mi equipo of the lead: Mario and the viewer, who is free and goes first,
	// with Mario's work of every project (RF-21 to RF-24).
	var team struct {
		Today string `json:"today"`
		Teams []struct {
			Name string `json:"name"`
		} `json:"teams"`
		People []workloadRowDTO `json:"people"`
	}
	s.exec(t, "UPDATE users SET weekly_capacity = 2 WHERE id = $1", s.memberID)
	s.lead.Get("/api/me/team?today=2026-03-10").Expect(t, http.StatusOK).JSON(t, &team)
	if team.Today != "2026-03-10" || len(team.People) != 2 || len(team.Teams) != 2 {
		t.Fatalf("Mi equipo = %+v", team)
	}
	viewer, mario := team.People[0], team.People[1]
	if viewer.User.Name != "viewer@x.com" || viewer.FreeFrom != nil || len(viewer.Bars) != 0 || len(viewer.Teams) != 1 || viewer.Teams[0].Name != "BI" {
		t.Fatalf("observador = %+v", viewer)
	}
	if mario.FreeFrom == nil || *mario.FreeFrom != "2026-04-02" || mario.Open != 4 || len(mario.Bars) != 3 ||
		mario.Capacity == nil || *mario.Capacity != 2 || !mario.Overloaded || mario.Teams[0].Name != "BI Reportes" {
		t.Fatalf("Mario en Mi equipo = %+v", mario)
	}

	// The admin sees everybody else; anybody else, nothing (RF-20).
	s.admin.Get("/api/me/team").Expect(t, http.StatusOK).JSON(t, &team)
	if len(team.People) != 4 {
		t.Fatalf("Mi equipo del admin = %+v", team.People)
	}
	s.member.Get("/api/me/team").Expect(t, http.StatusForbidden)
	s.admin.Get("/api/me/team?today=ayer").Expect(t, http.StatusBadRequest)
}
