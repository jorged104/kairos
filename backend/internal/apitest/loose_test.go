package apitest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// Spec 022: tareas sin proyecto.

type looseDTO struct {
	ID        uuid.UUID `json:"id"`
	Key       string    `json:"key"`
	Title     string    `json:"title"`
	Version   int32     `json:"version"`
	StatusID  uuid.UUID `json:"status_id"`
	ClosedAt  *string   `json:"closed_at"`
	Assignees []struct {
		ID uuid.UUID `json:"id"`
	} `json:"assignees"`
	Project struct {
		ID         uuid.UUID `json:"id"`
		Identifier string    `json:"identifier"`
	} `json:"project"`
	Can struct {
		Write  bool `json:"write"`
		Delete bool `json:"delete"`
	} `json:"can"`
	Loose *struct {
		Team *struct {
			Name string `json:"name"`
		} `json:"team"`
		Mode  string `json:"mode"`
		Done  int    `json:"done"`
		Total int    `json:"total"`
		Can   struct {
			Claim   bool `json:"claim"`
			Release bool `json:"release"`
			Move    bool `json:"move"`
		} `json:"can"`
	} `json:"loose"`
}

func (s *workSetup) loose(t *testing.T, c *Client, body map[string]any, status int) looseDTO {
	t.Helper()
	if _, ok := body["title"]; !ok {
		body["title"] = "Suelta"
	}
	var out looseDTO
	r := c.Post("/api/loose-tasks", body).Expect(t, status)
	if status == http.StatusCreated {
		r.JSON(t, &out)
	}
	return out
}

func (s *workSetup) looseGet(t *testing.T, c *Client, id uuid.UUID) looseDTO {
	t.Helper()
	var out looseDTO
	c.Get("/api/work-items/"+id.String()).Expect(t, http.StatusOK).JSON(t, &out)
	return out
}

func TestLoosePersonalTasks(t *testing.T) {
	s := newWorkSetup(t)

	// One's own, with the hidden project's key (RF-1 to RF-4, RF-10).
	own := s.loose(t, s.member, map[string]any{"title": "Mía", "user_id": s.memberID, "due_date": "2026-03-20"}, http.StatusCreated)
	if own.Key != "TAREA-1" || own.Loose == nil || own.Loose.Mode != "" || !own.Can.Write || !own.Can.Delete || !own.Loose.Can.Move ||
		len(own.Assignees) != 1 || own.Assignees[0].ID != s.memberID {
		t.Fatalf("propia = %+v", own)
	}
	// Not for somebody else, nor for a viewer (RF-4, RF-6).
	s.loose(t, s.member, map[string]any{"user_id": s.outsiderID}, http.StatusNotFound)
	s.loose(t, s.viewer, map[string]any{"user_id": s.memberID}, http.StatusForbidden)
	s.loose(t, s.member, map[string]any{}, http.StatusBadRequest)

	// The lead for their people; the admin for anybody (RF-4).
	fromLead := s.loose(t, s.lead, map[string]any{"title": "Del líder", "user_id": s.memberID}, http.StatusCreated)
	s.loose(t, s.lead, map[string]any{"user_id": s.outsiderID}, http.StatusNotFound)
	s.loose(t, s.admin, map[string]any{"user_id": s.outsiderID}, http.StatusCreated)

	// Who sees and changes it (RF-14 to RF-17): Otto doesn't; the lead
	// reaches Mario's; Mario can't delete what the lead gave him.
	s.outsider.Get("/api/work-items/"+own.ID.String()).Expect(t, http.StatusNotFound)
	s.outsider.Get("/api/work-items/by-key/TAREA-1").Expect(t, http.StatusNotFound)
	if got := s.looseGet(t, s.lead, own.ID); !got.Can.Write || got.Can.Delete {
		t.Fatalf("Mía vista por el líder = %+v", got.Can)
	}
	if got := s.looseGet(t, s.member, fromLead.ID); !got.Can.Write || got.Can.Delete {
		t.Fatalf("del líder vista por Mario = %+v", got.Can)
	}
	s.member.Do(http.MethodDelete, "/api/work-items/"+fromLead.ID.String(), nil).Expect(t, http.StatusForbidden)

	// The hidden project is nobody's project (RF-40).
	s.admin.Get("/api/projects/"+own.Project.ID.String()).Expect(t, http.StatusNotFound)
	s.admin.Get("/api/projects/"+own.Project.ID.String()+"/tree").Expect(t, http.StatusNotFound)
	s.admin.Post("/api/projects/"+own.Project.ID.String()+"/work-items", map[string]any{"title": "x", "type_id": s.env.CatalogID("work_item_types", "Tarea")}).
		Expect(t, http.StatusNotFound)
	var list struct {
		Items []struct {
			Identifier string `json:"identifier"`
		} `json:"items"`
	}
	s.admin.Get("/api/projects").Expect(t, http.StatusOK).JSON(t, &list)
	for _, p := range list.Items {
		if p.Identifier == "TAREA" {
			t.Fatal("la bandeja sale en Proyectos")
		}
	}
	if b := string(s.admin.Get("/api/portfolio").Expect(t, http.StatusOK).Body); strings.Contains(b, "TAREA") {
		t.Fatal("la bandeja sale en el Portafolio")
	}
	if b := string(s.admin.Get("/api/tracking").Expect(t, http.StatusOK).Body); strings.Contains(b, "TAREA") {
		t.Fatal("la bandeja sale en Seguimiento")
	}

	// Mi panel: marked, and no project for it (RF-29 to RF-32).
	var d dashboardDTO
	s.member.Get("/api/me/dashboard?today=2026-03-10").Expect(t, http.StatusOK).JSON(t, &d)
	if len(d.Tasks) != 2 || len(d.Projects) != 0 || len(d.Load) != 0 || d.Profile.Open != 2 {
		t.Fatalf("panel = %+v", d)
	}
	if b := string(s.member.Get("/api/me/dashboard").Expect(t, http.StatusOK).Body); !strings.Contains(b, `"loose":{"team":null,"mode":""}`) {
		t.Fatalf("panel sin marca: %s", b)
	}

	// Closing it pays like any work item (RF-13).
	s.moveItem(t, own.ID, s.env.CatalogID("statuses", "Cerrado"), nil)
	if got := points(t, s.member); got.XP == 0 {
		t.Fatalf("puntos = %+v", got)
	}

	// To a project where Mario writes, with its key and its comments (RF-37, RF-38).
	p := s.project(t, s.lead, map[string]any{"identifier": "ALF", "name": "Alfa"})
	s.comment(t, s.member, fromLead.ID, text("Hola"), http.StatusCreated)
	var moved looseDTO
	s.member.Post("/api/work-items/"+fromLead.ID.String()+"/move", map[string]any{"project_id": p.ID, "version": fromLead.Version}).
		Expect(t, http.StatusOK).JSON(t, &moved)
	if moved.Key != "ALF-1" || moved.Loose != nil || moved.Project.ID != p.ID {
		t.Fatalf("movida = %+v", moved)
	}
	if got := s.comments(t, s.member, fromLead.ID); len(got.Threads) != 1 {
		t.Fatalf("comentarios tras mover = %+v", got)
	}
	s.member.Post("/api/work-items/"+fromLead.ID.String()+"/move", map[string]any{"project_id": p.ID, "version": moved.Version}).
		Expect(t, http.StatusConflict)
}

func TestLooseClaimTasks(t *testing.T) {
	s := newWorkSetup(t)
	anaID := s.env.CreateUser(UserSpec{Name: "Ana", Email: "ana@x.com", Roles: []string{"member"}, Teams: []uuid.UUID{s.bi}})
	ana := s.env.As(anaID)

	// For a team the lead leads; not for another (RF-5, RF-6).
	task := s.loose(t, s.lead, map[string]any{"title": "Incidencia", "team_id": s.bi, "mode": "claim"}, http.StatusCreated)
	s.loose(t, s.lead, map[string]any{"team_id": s.other, "mode": "claim"}, http.StatusNotFound)
	s.loose(t, s.lead, map[string]any{"team_id": s.bi}, http.StatusBadRequest)
	if task.Loose == nil || task.Loose.Mode != "claim" || task.Loose.Team.Name != "BI" || len(task.Assignees) != 0 {
		t.Fatalf("tarea del equipo = %+v", task)
	}

	// Mario (a subteam) and Ana see it to take; Otto doesn't (RF-18).
	var claimable struct {
		Items []struct {
			ID uuid.UUID `json:"id"`
		} `json:"items"`
	}
	s.member.Get("/api/me/claimable-tasks").Expect(t, http.StatusOK).JSON(t, &claimable)
	if len(claimable.Items) != 1 || claimable.Items[0].ID != task.ID {
		t.Fatalf("para tomar = %+v", claimable)
	}
	s.outsider.Get("/api/me/claimable-tasks").Expect(t, http.StatusOK).JSON(t, &claimable)
	if len(claimable.Items) != 0 {
		t.Fatalf("para tomar de Otto = %+v", claimable)
	}
	s.outsider.Post("/api/work-items/"+task.ID.String()+"/claim", nil).Expect(t, http.StatusNotFound)
	if got := s.looseGet(t, ana, task.ID); !got.Loose.Can.Claim || got.Can.Write {
		t.Fatalf("vista de Ana = %+v", got)
	}

	// Mario takes it; then Ana can't (RF-19 to RF-21).
	var taken looseDTO
	s.member.Post("/api/work-items/"+task.ID.String()+"/claim", nil).Expect(t, http.StatusOK).JSON(t, &taken)
	if len(taken.Assignees) != 1 || taken.Assignees[0].ID != s.memberID || !taken.Loose.Can.Release || !taken.Can.Write {
		t.Fatalf("tomada = %+v", taken)
	}
	ana.Post("/api/work-items/"+task.ID.String()+"/claim", nil).Expect(t, http.StatusConflict)
	ana.Post("/api/work-items/"+task.ID.String()+"/release", nil).Expect(t, http.StatusConflict)
	s.member.Get("/api/me/claimable-tasks").Expect(t, http.StatusOK).JSON(t, &claimable)
	if len(claimable.Items) != 0 {
		t.Fatalf("para tomar tras tomarla = %+v", claimable)
	}

	// Mario gives it back; it is there to take again (RF-22).
	s.member.Post("/api/work-items/"+task.ID.String()+"/release", nil).Expect(t, http.StatusOK)
	ana.Get("/api/me/claimable-tasks").Expect(t, http.StatusOK).JSON(t, &claimable)
	if len(claimable.Items) != 1 {
		t.Fatalf("para tomar tras devolverla = %+v", claimable)
	}

	// The lead gives it to Ana (RF-23), and sees it in Mi equipo (RF-33, RF-34).
	cur := s.get(t, task.ID)
	s.patch(s.lead, cur, map[string]any{"assignee_ids": []uuid.UUID{anaID}}).Expect(t, http.StatusOK)
	s.patch(s.lead, s.get(t, task.ID), map[string]any{"assignee_ids": []uuid.UUID{s.outsiderID}}).Expect(t, http.StatusNotFound)
	var tt struct {
		Items []struct {
			Key    string `json:"key"`
			Mode   string `json:"mode"`
			People []struct {
				Name string `json:"name"`
			} `json:"people"`
		} `json:"items"`
	}
	s.lead.Get("/api/me/team-tasks").Expect(t, http.StatusOK).JSON(t, &tt)
	if len(tt.Items) != 1 || tt.Items[0].Mode != "claim" || len(tt.Items[0].People) != 1 || tt.Items[0].People[0].Name != "Ana" {
		t.Fatalf("tareas del equipo = %+v", tt)
	}
	s.member.Get("/api/me/team-tasks").Expect(t, http.StatusForbidden)
}

func TestLooseEachTasks(t *testing.T) {
	s := newWorkSetup(t)
	anaID := s.env.CreateUser(UserSpec{Name: "Ana", Email: "ana@x.com", Roles: []string{"member"}, Teams: []uuid.UUID{s.bi}})
	ana := s.env.As(anaID)
	empty := s.env.CreateTeam("Vacío", &s.bi)

	// Nobody to do it (RF-8).
	s.loose(t, s.lead, map[string]any{"team_id": empty, "mode": "each"}, http.StatusConflict)

	// One for Mario and one for Ana; not for the viewer (RF-9).
	group := s.loose(t, s.lead, map[string]any{"title": "Formación", "team_id": s.bi, "mode": "each", "due_date": "2026-03-30"}, http.StatusCreated)
	if group.Loose.Mode != "each" || group.Loose.Total != 2 || group.Loose.Done != 0 || len(group.Assignees) != 0 || group.Loose.Can.Move {
		t.Fatalf("grupo = %+v", group)
	}
	var d dashboardDTO
	s.member.Get("/api/me/dashboard").Expect(t, http.StatusOK).JSON(t, &d)
	if len(d.Tasks) != 1 || d.Tasks[0].Title != "Formación" {
		t.Fatalf("panel de Mario = %+v", d.Tasks)
	}
	mine := d.Tasks[0].ID

	// Someone who enters the team gets theirs (RF-25).
	evaID := s.env.CreateUser(UserSpec{Name: "Eva", Email: "eva@x.com", Roles: []string{"member"}})
	s.admin.Post("/api/users/"+evaID.String()+"/teams", map[string]any{"team_id": s.bi}).Expect(t, http.StatusOK)
	if got := s.looseGet(t, s.lead, group.ID); got.Loose.Total != 3 {
		t.Fatalf("tras entrar Eva = %+v", got.Loose)
	}

	// Closing them all closes the group (RF-24, RF-27); a child can't move (RF-39).
	cerrado := s.env.CatalogID("statuses", "Cerrado")
	s.moveItem(t, mine, cerrado, nil)
	if got := s.looseGet(t, s.member, mine); got.Loose.Done != 1 || got.Loose.Total != 3 || got.Loose.Can.Move {
		t.Fatalf("hija de Mario = %+v", got.Loose)
	}
	s.member.Post("/api/work-items/"+mine.String()+"/move", map[string]any{"project_id": uuid.New(), "version": 1}).Expect(t, http.StatusConflict)
	ana.Get("/api/me/dashboard").Expect(t, http.StatusOK).JSON(t, &d)
	s.moveItem(t, d.Tasks[0].ID, cerrado, nil)
	if got := s.looseGet(t, s.lead, group.ID); got.ClosedAt != nil {
		t.Fatalf("grupo cerrado antes de tiempo = %+v", got)
	}

	// Deleting the group takes the open ones and keeps the closed (RF-28).
	s.lead.Do(http.MethodDelete, "/api/work-items/"+group.ID.String(), nil).Expect(t, http.StatusNoContent)
	if got := s.looseGet(t, s.member, mine); got.Loose.Mode != "" || got.ClosedAt == nil {
		t.Fatalf("cerrada tras borrar el grupo = %+v", got)
	}
	var count int
	if err := s.env.Store.Pool.QueryRow(t.Context(), "SELECT count(*) FROM work_items w JOIN work_item_assignees a ON a.work_item_id = w.id WHERE a.user_id = $1", evaID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("tareas de Eva = %d, %v", count, err)
	}

	// A group closes by itself when the last child closes.
	g2 := s.loose(t, s.lead, map[string]any{"title": "Firma", "team_id": s.rep, "mode": "each"}, http.StatusCreated)
	s.member.Get("/api/me/dashboard").Expect(t, http.StatusOK).JSON(t, &d)
	for _, x := range d.Tasks {
		if x.Title == "Firma" {
			s.moveItem(t, x.ID, cerrado, nil)
		}
	}
	if got := s.looseGet(t, s.lead, g2.ID); got.ClosedAt == nil || got.Loose.Done != 1 || got.Loose.Total != 1 {
		t.Fatalf("grupo de una persona = %+v", got)
	}
}
