package apitest

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

type projectStatusDTO struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	IsClosed  bool      `json:"is_closed"`
	IsInitial bool      `json:"is_initial"`
	IsClosing bool      `json:"is_closing"`
	Retired   bool      `json:"retired"`
}

type statusedProjectDTO struct {
	Version   int32      `json:"version"`
	Lifecycle string     `json:"lifecycle"`
	StatusID  *uuid.UUID `json:"status_id"`
}

type boardDTO struct {
	Today    string `json:"today"`
	Projects []struct {
		ID         uuid.UUID  `json:"id"`
		Name       string     `json:"name"`
		StatusID   uuid.UUID  `json:"status_id"`
		IsProgram  bool       `json:"is_program"`
		CanManage  bool       `json:"can_manage"`
		BaseStart  *string    `json:"base_start"`
		BaseEnd    *string    `json:"base_end"`
		Figures    figuresDTO `json:"figures"`
		Milestones []struct {
			Name string `json:"name"`
			Done bool   `json:"done"`
		} `json:"milestones"`
		ProgramIDs []uuid.UUID `json:"program_ids"`
		Program    *struct {
			ID   uuid.UUID `json:"id"`
			Name string    `json:"name"`
		} `json:"program"`
		Owner struct {
			Name string `json:"name"`
		} `json:"owner"`
	} `json:"projects"`
	Programs []struct {
		ID      uuid.UUID  `json:"id"`
		Figures figuresDTO `json:"figures"`
	} `json:"programs"`
}

type listViewDTO struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	Shared  bool      `json:"shared"`
	Mine    bool      `json:"mine"`
	CanEdit bool      `json:"can_edit"`
	Config  struct {
		Mode    string              `json:"mode"`
		Filters map[string][]string `json:"filters"`
		Columns string              `json:"columns"`
	} `json:"config"`
}

func (s *workSetup) projectStatuses(t *testing.T) map[string]projectStatusDTO {
	t.Helper()
	var c struct {
		ProjectStatuses []projectStatusDTO `json:"project_statuses"`
	}
	s.admin.Get("/api/catalog").Expect(t, http.StatusOK).JSON(t, &c)
	out := map[string]projectStatusDTO{}
	for _, st := range c.ProjectStatuses {
		out[st.Name] = st
	}
	return out
}

func (s *workSetup) statused(t *testing.T, id uuid.UUID) statusedProjectDTO {
	t.Helper()
	var p statusedProjectDTO
	s.admin.Get("/api/projects/"+id.String()).Expect(t, http.StatusOK).JSON(t, &p)
	return p
}

func (s *workSetup) setStatus(c *Client, id uuid.UUID, version int32, status uuid.UUID) *Response {
	return c.Do(http.MethodPut, "/api/projects/"+id.String()+"/status", map[string]any{"version": version, "status_id": status})
}

func TestProjectStatusCatalog(t *testing.T) {
	s := newWorkSetup(t)
	st := s.projectStatuses(t)
	// The five neutral statuses (RF-12).
	if len(st) != 5 || !st["Propuesta"].IsInitial || !st["Cerrado"].IsClosing || !st["Cerrado"].IsClosed || st["En pausa"].IsClosed {
		t.Fatalf("seed = %+v", st)
	}
	body := func(name string, closed, initial, closing bool) map[string]any {
		return map[string]any{"name": name, "color": "#123456", "position": 9, "is_closed": closed, "is_initial": initial, "is_closing": closing}
	}
	var created projectStatusDTO
	s.admin.Post("/api/project-statuses", body("Revisión", false, false, false)).Expect(t, http.StatusCreated).JSON(t, &created) // RF-1
	s.lead.Post("/api/project-statuses", body("Otro", false, false, false)).Expect(t, http.StatusForbidden)
	s.admin.Post("/api/project-statuses", body(" revisión ", false, false, false)).Expect(t, http.StatusConflict, "catalog_name_conflict") // RF-8
	// The initial status is open and the closing one closed (RF-4, RF-5).
	s.admin.Post("/api/project-statuses", body("Mal", true, true, false)).Expect(t, http.StatusBadRequest)
	s.admin.Post("/api/project-statuses", body("Mal", false, false, true)).Expect(t, http.StatusBadRequest)

	// The marks move but never go away (RF-6, RF-7).
	path := "/api/project-statuses/"
	s.admin.Do(http.MethodPatch, path+st["Propuesta"].ID.String(), body("Propuesta", false, false, false)).Expect(t, http.StatusConflict, "catalog_default_required")
	s.admin.Post(path+st["Propuesta"].ID.String()+"/retire", nil).Expect(t, http.StatusConflict, "catalog_default_required")
	s.admin.Post(path+st["Cerrado"].ID.String()+"/retire", nil).Expect(t, http.StatusConflict, "catalog_default_required")
	s.admin.Do(http.MethodPatch, path+created.ID.String(), body("Revisión", false, true, false)).Expect(t, http.StatusOK) // RF-4
	if now := s.projectStatuses(t); now["Propuesta"].IsInitial || !now["Revisión"].IsInitial {
		t.Fatalf("initial did not move = %+v", now)
	}
	p := s.project(t, s.lead, map[string]any{"identifier": "NUE", "name": "Nuevo"})
	if got := s.statused(t, p.ID); got.StatusID == nil || *got.StatusID != created.ID { // RF-14
		t.Fatalf("new project status = %+v", got)
	}

	// A status in use keeps its category, and its projects keep it when retired (RF-9).
	s.admin.Do(http.MethodPatch, path+st["En pausa"].ID.String(), body("En pausa", false, false, false)).Expect(t, http.StatusOK)
	s.setStatus(s.lead, p.ID, s.statused(t, p.ID).Version, st["En pausa"].ID).Expect(t, http.StatusOK)
	s.admin.Do(http.MethodPatch, path+st["En pausa"].ID.String(), body("En pausa", true, false, false)).Expect(t, http.StatusConflict, "status_closed_locked")
	s.admin.Post(path+st["En pausa"].ID.String()+"/retire", nil).Expect(t, http.StatusOK)
	if got := s.statused(t, p.ID); *got.StatusID != st["En pausa"].ID {
		t.Fatalf("retired status left the project = %+v", got)
	}
	// A retired status cannot be chosen (RF-10).
	s.setStatus(s.lead, p.ID, s.statused(t, p.ID).Version, st["En pausa"].ID).Expect(t, http.StatusBadRequest)
	if acts := s.env.AuditActions(created.ID.String()); len(acts) != 2 || acts[0] != "catalog.project_status.created" { // RF-11
		t.Fatalf("audit = %v", acts)
	}
}

func TestProjectStatusChange(t *testing.T) {
	s := newWorkSetup(t)
	st := s.projectStatuses(t)
	p := s.project(t, s.lead, map[string]any{"identifier": "EST", "name": "Estados", "owner_id": s.memberID})
	cur := s.statused(t, p.ID)

	// Whoever manages it moves it freely (RF-17, RF-18).
	s.setStatus(s.outsider, p.ID, cur.Version, st["En ejecución"].ID).Expect(t, http.StatusNotFound) // does not see the project (spec 023)
	s.setStatus(s.viewer, p.ID, cur.Version, st["En ejecución"].ID).Expect(t, http.StatusForbidden)
	s.setStatus(s.member, p.ID, cur.Version, st["En ejecución"].ID).Expect(t, http.StatusOK).JSON(t, &cur)
	if cur.Lifecycle != "activo" || *cur.StatusID != st["En ejecución"].ID {
		t.Fatalf("open move = %+v", cur)
	}
	s.setStatus(s.member, p.ID, cur.Version-1, st["Cerrado"].ID).Expect(t, http.StatusConflict, "version_conflict") // RF-24

	// Into a closed status it closes, out of it it reopens (RF-19, RF-20).
	s.setStatus(s.member, p.ID, cur.Version, st["Cerrado"].ID).Expect(t, http.StatusOK).JSON(t, &cur)
	if cur.Lifecycle != "cerrado" {
		t.Fatalf("did not close = %+v", cur)
	}
	s.setStatus(s.member, p.ID, cur.Version, st["Planificación"].ID).Expect(t, http.StatusOK).JSON(t, &cur)
	if cur.Lifecycle != "activo" {
		t.Fatalf("did not reopen = %+v", cur)
	}
	acts := s.env.AuditActions(p.ID.String())
	want := []string{"project.status_changed", "project.close", "project.status_changed", "project.reopen", "project.status_changed"}
	if n := len(acts); n < len(want) || !equal(acts[n-len(want):], want) { // RF-25
		t.Fatalf("audit = %v", acts)
	}

	// Closing and reopening from the file move the status (RF-21, RF-22).
	s.member.Post("/api/projects/"+p.ID.String()+"/close", map[string]any{"version": cur.Version}).Expect(t, http.StatusOK).JSON(t, &cur)
	if *cur.StatusID != st["Cerrado"].ID {
		t.Fatalf("close kept %v", cur.StatusID)
	}
	s.member.Post("/api/projects/"+p.ID.String()+"/reopen", map[string]any{"version": cur.Version}).Expect(t, http.StatusOK).JSON(t, &cur)
	if *cur.StatusID != st["Propuesta"].ID {
		t.Fatalf("reopen kept %v", cur.StatusID)
	}
	// An archived project keeps its status and cannot change it (RF-23).
	s.lead.Post("/api/projects/"+p.ID.String()+"/archive", map[string]any{"version": cur.Version}).Expect(t, http.StatusOK).JSON(t, &cur)
	s.setStatus(s.lead, p.ID, cur.Version, st["En ejecución"].ID).Expect(t, http.StatusConflict)
	if *cur.StatusID != st["Propuesta"].ID {
		t.Fatalf("archive changed the status: %v", cur.StatusID)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// A project left without status by the previous API reads as the initial one (RF-13).
func TestProjectStatusNull(t *testing.T) {
	s := newWorkSetup(t)
	st := s.projectStatuses(t)
	p := s.project(t, s.lead, map[string]any{"identifier": "NUL", "name": "Nulo"})
	if _, err := s.env.Store.Pool.Exec(context.Background(), "UPDATE projects SET status_id = NULL WHERE id = $1", p.ID); err != nil {
		t.Fatal(err)
	}
	if got := s.statused(t, p.ID); got.StatusID == nil || *got.StatusID != st["Propuesta"].ID {
		t.Fatalf("null status = %+v", got)
	}
	var cur statusedProjectDTO
	s.lead.Post("/api/projects/"+p.ID.String()+"/close", map[string]any{"version": s.statused(t, p.ID).Version}).Expect(t, http.StatusOK).JSON(t, &cur)
	if *cur.StatusID != st["Cerrado"].ID {
		t.Fatalf("close from null = %+v", cur)
	}
}

func TestProjectBoard(t *testing.T) {
	s := newWorkSetup(t)
	prog := s.project(t, s.lead, map[string]any{"identifier": "PRG", "name": "Programa"})
	s.setPortfolio(s.admin, prog.ID, prog.Version, true, nil).Expect(t, http.StatusOK)
	mid := s.project(t, s.lead, map[string]any{"identifier": "MID", "name": "Medio", "parent_id": prog.ID})
	leaf := s.project(t, s.lead, map[string]any{"identifier": "HOJ", "name": "Hoja", "parent_id": mid.ID,
		"start_date": "2026-03-01", "end_date": "2026-03-31"})
	s.item(t, s.member, leaf.ID, map[string]any{"weight": 1, "start_date": "2026-03-01", "due_date": "2026-03-20"})
	s.lead.Post("/api/projects/"+leaf.ID.String()+"/baseline", map[string]any{}).Expect(t, http.StatusCreated)
	s.milestone(t, leaf.ID, map[string]any{"name": "Entrega", "due_date": "2026-03-15"})
	s.milestone(t, leaf.ID, map[string]any{"name": "Sin fecha"})
	gone := s.project(t, s.lead, map[string]any{"identifier": "ARC", "name": "Archivado"})
	s.lead.Post("/api/projects/"+gone.ID.String()+"/archive", map[string]any{"version": gone.Version}).Expect(t, http.StatusOK)
	s.project(t, s.admin, map[string]any{"identifier": "OTR", "name": "Otro", "team_id": s.other})

	var b boardDTO
	s.member.Get("/api/project-board?today=2026-03-10").Expect(t, http.StatusOK).JSON(t, &b)
	byName := map[string]int{}
	for i, p := range b.Projects {
		byName[p.Name] = i
	}
	// Neither archived (RF-29) nor of a team the member does not see (spec 023, RF-24).
	if _, ok := byName["Archivado"]; ok || len(b.Projects) != 3 || b.Today != "2026-03-10" {
		t.Fatalf("board projects = %+v", b.Projects)
	}
	h := b.Projects[byName["Hoja"]]
	if len(h.ProgramIDs) != 1 || h.Program == nil || h.Program.ID != prog.ID || b.Projects[byName["Medio"]].Program == nil || b.Projects[byName["Programa"]].Program != nil {
		t.Fatalf("nearest program = %+v", h.Program)
	}
	if !h.Figures.HasPlan || h.BaseStart == nil || *h.BaseStart != "2026-03-01" || *h.BaseEnd != "2026-03-20" { // RF-36, RF-49
		t.Fatalf("leaf = %+v", h)
	}
	if len(h.Milestones) != 1 || h.Milestones[0].Name != "Entrega" { // RF-50: only the dated ones
		t.Fatalf("milestones = %+v", h.Milestones)
	}
	if h.CanManage {
		t.Fatalf("member manages the lead's project")
	}
	var lb boardDTO
	s.lead.Get("/api/project-board").Expect(t, http.StatusOK).JSON(t, &lb)
	leadSees := map[string]bool{}
	for _, p := range lb.Projects {
		leadSees[p.Name] = p.CanManage
	}
	if manage, ok := leadSees["Hoja"]; !ok || !manage {
		t.Fatalf("lead can manage = %+v", lb.Projects)
	}
	if _, ok := leadSees["Otro"]; ok { // spec 023, RF-24
		t.Fatalf("the lead sees a project of another team")
	}
	var ab boardDTO
	s.admin.Get("/api/project-board").Expect(t, http.StatusOK).JSON(t, &ab)
	if len(ab.Projects) != 4 {
		t.Fatalf("admin board = %d projects", len(ab.Projects))
	}
	if len(b.Programs) != 1 || b.Programs[0].ID != prog.ID || !b.Programs[0].Figures.HasPlan { // RF-52
		t.Fatalf("programs = %+v", b.Programs)
	}
	s.member.Get("/api/project-board?today=mañana").Expect(t, http.StatusBadRequest)
}

func TestProjectListViews(t *testing.T) {
	s := newWorkSetup(t)
	save := func(c *Client, name string, shared bool, cfg map[string]any) *Response {
		return c.Post("/api/project-views", map[string]any{"name": name, "shared": shared, "config": cfg})
	}
	list := func(c *Client) []listViewDTO {
		var out struct {
			Items []listViewDTO `json:"items"`
		}
		c.Get("/api/project-views").Expect(t, http.StatusOK).JSON(t, &out)
		return out.Items
	}
	cfg := map[string]any{"mode": "kanban", "filters": map[string][]string{"owner_id": {"me"}, "level": {"late"}}, "sort": "gap", "columns": "status"}
	var mine listViewDTO
	save(s.member, "Mis atrasados", false, cfg).Expect(t, http.StatusCreated).JSON(t, &mine) // RF-70
	if !mine.Mine || mine.Shared || !mine.CanEdit || mine.Config.Filters["owner_id"][0] != "me" {
		t.Fatalf("personal = %+v", mine)
	}
	save(s.member, "Compartida", true, cfg).Expect(t, http.StatusForbidden) // RF-71
	save(s.member, " MIS ATRASADOS ", false, cfg).Expect(t, http.StatusConflict, "view_name_conflict")
	save(s.member, "Todos", false, cfg).Expect(t, http.StatusConflict, "view_name_conflict") // RF-69
	for _, bad := range []map[string]any{
		{"mode": "lista"}, {"filters": map[string][]string{"color": {"x"}}}, {"filters": map[string][]string{"level": {"rojo"}}},
		{"filters": map[string][]string{"status_id": {"no"}}}, {"sort": "edad"}, {"columns": "team"}, {"group": "mes"},
	} {
		save(s.member, "Mala", false, bad).Expect(t, http.StatusBadRequest)
	}
	var shared listViewDTO
	save(s.lead, "Del equipo", true, map[string]any{"mode": "timeline", "group": "program"}).Expect(t, http.StatusCreated).JSON(t, &shared)

	// Each person sees the shared views and their own (RF-72).
	if got := list(s.member); len(got) != 2 || !got[0].Shared || got[0].CanEdit || got[1].ID != mine.ID {
		t.Fatalf("member list = %+v", got)
	}
	if got := list(s.outsider); len(got) != 1 {
		t.Fatalf("outsider list = %+v", got)
	}
	// Only the author changes a personal view; a shared one, who shares (RF-75).
	path := "/api/project-views/"
	s.outsider.Do(http.MethodPatch, path+mine.ID.String(), map[string]any{"name": "X", "config": cfg}).Expect(t, http.StatusNotFound)
	s.member.Do(http.MethodPatch, path+shared.ID.String(), map[string]any{"name": "X", "shared": true, "config": cfg}).Expect(t, http.StatusForbidden)
	s.admin.Do(http.MethodPatch, path+shared.ID.String(), map[string]any{"name": "Del área", "shared": true, "config": cfg}).Expect(t, http.StatusOK)
	s.member.Do(http.MethodPatch, path+mine.ID.String(), map[string]any{"name": "Mis atrasados", "config": map[string]any{"mode": "table"}}).Expect(t, http.StatusOK)
	s.member.Do(http.MethodDelete, path+shared.ID.String(), nil).Expect(t, http.StatusForbidden)
	s.lead.Do(http.MethodDelete, path+shared.ID.String(), nil).Expect(t, http.StatusNoContent)
	// Only shared views are audited (RF-80).
	if acts := s.env.AuditActions(shared.ID.String()); !equal(acts, []string{"project_list_view.created", "project_list_view.updated", "project_list_view.deleted"}) {
		t.Fatalf("shared audit = %v", acts)
	}
	if acts := s.env.AuditActions(mine.ID.String()); len(acts) != 0 {
		t.Fatalf("personal audit = %v", acts)
	}

	// At most 50 personal views (RF-77).
	if _, err := s.env.Store.Pool.Exec(context.Background(),
		`INSERT INTO project_list_views (owner_id, name, name_normalized, created_by)
		 SELECT $1, 'v' || g, 'v' || g, $1 FROM generate_series(1, 49) g`, s.memberID); err != nil {
		t.Fatal(err)
	}
	save(s.member, "Una más", false, cfg).Expect(t, http.StatusConflict, "too_many_views")
}
