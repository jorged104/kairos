// Package dashboard serves Mi panel: the caller's work items, the projects
// they work on and the series of their progress (spec 013). Besides their
// own, an admin or a lead may see, read only, the panel of the people they
// can (change of 2026-11-06, RF-37 to RF-51).
package dashboard

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/auth"
	"openscum/internal/authz"
	"openscum/internal/db/sqlc"
	"openscum/internal/portfolio"
	"openscum/internal/schedule"
	"openscum/internal/workcore"
	"openscum/internal/workload"
	"openscum/pkg/httpx"
)

const (
	weeks      = 12 // of the series (RF-28)
	closedDays = 14 // of the closed columns (RF-11)
	maxLoad    = 8  // projects of the load before «Otros» (RF-34)
)

type Ref struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type Task struct {
	ID       uuid.UUID `json:"id"`
	Key      string    `json:"key"`
	Title    string    `json:"title"`
	Project  Ref       `json:"project"`
	StatusID uuid.UUID `json:"status_id"`
	// The type, whose colour marks the card (change of 2026-11-05, RF-14a).
	TypeID   uuid.UUID      `json:"type_id"`
	Closed   bool           `json:"closed"`
	DueDate  *workcore.Date `json:"due_date"`
	ClosedOn *workcore.Date `json:"closed_on"`
	LateDays int            `json:"late_days"`
	Weight   int16          `json:"weight"`
	CanMove  bool           `json:"can_move"`
}

type Profile struct {
	Open        int `json:"open"`
	Late        int `json:"late"`
	ClosedMonth int `json:"closed_month"`
}

type Project struct {
	ID         uuid.UUID        `json:"id"`
	Identifier string           `json:"identifier"`
	Name       string           `json:"name"`
	Role       string           `json:"role"` // owner, pm, tech_lead, member, observer or tasks (RF-24)
	Open       int              `json:"open"`
	Late       int              `json:"late"`
	Figures    schedule.Figures `json:"figures"`
}

type Week struct {
	Start  workcore.Date `json:"start"` // Monday
	Closed int           `json:"closed"`
	Weight int           `json:"weight"`
	// Closed with a due date, and of those on time (RF-30).
	WithDue int `json:"with_due"`
	OnTime  int `json:"on_time"`
}

type Load struct {
	Project *Ref `json:"project"` // null: «Otros»
	Open    int  `json:"open"`
	Late    int  `json:"late"`
}

// Person is whose panel it is, when it is not the caller's (RF-41, RF-46).
type Person struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Roles []string  `json:"roles"`
	Teams []Ref     `json:"teams"`
}

type Dashboard struct {
	// Only on another person's panel.
	User     *Person   `json:"user,omitempty"`
	Today    string    `json:"today"`
	Profile  Profile   `json:"profile"`
	Tasks    []Task    `json:"tasks"`
	Projects []Project `json:"projects"`
	Weeks    []Week    `json:"weeks"`
	Load     []Load    `json:"load"`
}

// Loader reads a user's roles, teams and project roles, as at login.
type Loader func(ctx context.Context, id uuid.UUID) (*authz.Principal, error)

type Service struct {
	q    *sqlc.Queries
	load Loader
}

func NewService(q *sqlc.Queries, load Loader) *Service { return &Service{q: q, load: load} }

// People are the other active users whose panel the caller may see, by name
// (RF-37, RF-38).
func (s *Service) People(ctx context.Context, p *authz.Principal) ([]Ref, error) {
	out := []Ref{}
	if !p.Has(authz.DashboardOthersAll) && !p.Has(authz.DashboardOthersTeam) {
		return out, nil
	}
	rows, err := s.q.DashboardPeople(ctx)
	if err != nil {
		return nil, fmt.Errorf("personas del panel: %w", err)
	}
	for _, r := range rows {
		if r.ID != p.UserID && p.CanSeeDashboardOf(target(r.ID, r.Roles, r.TeamIds)) {
			out = append(out, Ref{ID: r.ID, Name: r.Name})
		}
	}
	return out, nil
}

// Member is a person of Mi equipo: their schedule, teams and load against
// capacity (spec 021, RF-21 to RF-24).
type Member struct {
	workload.Row
	Teams      []Ref  `json:"teams"`
	Capacity   *int32 `json:"capacity"`
	Overloaded bool   `json:"overloaded"`
}

type Team struct {
	Today  string   `json:"today"`
	Teams  []Ref    `json:"teams"` // to filter by (RF-26)
	People []Member `json:"people"`
}

// Team is Mi equipo: the people whose panel the caller may see, with their
// work items of every project (spec 021). Without the permissions, forbidden
// (RF-20).
func (s *Service) Team(ctx context.Context, p *authz.Principal, today time.Time) (Team, error) {
	if !p.Has(authz.DashboardOthersAll) && !p.Has(authz.DashboardOthersTeam) {
		return Team{}, apperr.ErrForbidden
	}
	rows, err := s.q.DashboardPeople(ctx)
	if err != nil {
		return Team{}, fmt.Errorf("personas del equipo: %w", err)
	}
	all, err := s.q.ListTeams(ctx)
	if err != nil {
		return Team{}, fmt.Errorf("equipos: %w", err)
	}
	names := map[uuid.UUID]string{}
	for _, t := range all {
		if t.ArchivedAt == nil {
			names[t.ID] = t.Name
		}
	}
	people := []workcore.Ref{}
	ids := []uuid.UUID{}
	teamsOf := map[uuid.UUID][]Ref{}
	used := map[uuid.UUID]bool{}
	for _, r := range rows {
		if r.ID == p.UserID || !p.CanSeeDashboardOf(target(r.ID, r.Roles, r.TeamIds)) {
			continue
		}
		people = append(people, workcore.Ref{ID: r.ID, Name: r.Name})
		ids = append(ids, r.ID)
		ts := []Ref{}
		for _, id := range r.TeamIds {
			if n, ok := names[id]; ok {
				ts = append(ts, Ref{ID: id, Name: n})
				used[id] = true
			}
		}
		sort.Slice(ts, func(i, j int) bool { return ts[i].Name < ts[j].Name })
		teamsOf[r.ID] = ts
	}
	out := Team{Today: day(today).Format(time.DateOnly), Teams: []Ref{}, People: []Member{}}
	for _, t := range all {
		if used[t.ID] {
			out.Teams = append(out.Teams, Ref{ID: t.ID, Name: t.Name})
		}
	}
	if len(ids) == 0 {
		return out, nil
	}
	caps, err := s.q.TeamPeople(ctx, ids)
	if err != nil {
		return Team{}, fmt.Errorf("capacidad: %w", err)
	}
	capOf := map[uuid.UUID]*int32{}
	for _, c := range caps {
		capOf[c.ID] = c.WeeklyCapacity
	}
	items, err := s.q.PersonScheduleItems(ctx, sqlc.PersonScheduleItemsParams{Ids: ids})
	if err != nil {
		return Team{}, fmt.Errorf("cronograma por persona: %w", err)
	}
	for _, r := range workload.Build(people, items, today) {
		c := capOf[r.User.ID]
		out.People = append(out.People, Member{Row: r, Teams: teamsOf[r.User.ID], Capacity: c, Overloaded: c != nil && r.Weight > int64(*c)})
	}
	return out, nil
}

func target(id uuid.UUID, roles []string, teams []uuid.UUID) authz.Target {
	t := authz.Target{UserID: id, TeamIDs: teams}
	for _, r := range roles {
		t.Roles = append(t.Roles, authz.Role(r))
	}
	return t
}

// GetFor builds the panel of a person, read only, as that person would see it
// (RF-39, RF-42, RF-43). Somebody the caller may not see, inactive or
// missing, is not found alike (RF-48, RF-49).
func (s *Service) GetFor(ctx context.Context, p *authz.Principal, id uuid.UUID, today time.Time) (Dashboard, error) {
	if id == p.UserID {
		return s.Get(ctx, p, today)
	}
	u, err := s.q.GetUser(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && u.Status != "active") {
		return Dashboard{}, apperr.ErrNotFound
	}
	if err != nil {
		return Dashboard{}, fmt.Errorf("leer usuario: %w", err)
	}
	roles, err := s.q.ListUserRoles(ctx, id)
	if err != nil {
		return Dashboard{}, fmt.Errorf("leer roles: %w", err)
	}
	teams, err := s.q.ListUserTeams(ctx, id)
	if err != nil {
		return Dashboard{}, fmt.Errorf("leer equipos: %w", err)
	}
	person := &Person{ID: u.ID, Name: u.Name, Roles: []string{}, Teams: []Ref{}}
	teamIDs := make([]uuid.UUID, 0, len(teams))
	for _, r := range roles {
		person.Roles = append(person.Roles, r.Role)
	}
	for _, t := range teams {
		teamIDs = append(teamIDs, t.ID)
		if t.ArchivedAt == nil {
			person.Teams = append(person.Teams, Ref{ID: t.ID, Name: t.Name})
		}
	}
	if !p.CanSeeDashboardOf(target(id, person.Roles, teamIDs)) {
		return Dashboard{}, apperr.ErrNotFound
	}
	them, err := s.load(ctx, id)
	if err != nil {
		return Dashboard{}, fmt.Errorf("cargar usuario del panel: %w", err)
	}
	out, err := s.Get(ctx, them, today)
	if err != nil {
		return out, err
	}
	for i := range out.Tasks {
		out.Tasks[i].CanMove = false
	}
	out.User = person
	return out, nil
}

func day(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC) }

// Get builds the caller's panel at their date today.
func (s *Service) Get(ctx context.Context, p *authz.Principal, today time.Time) (Dashboard, error) {
	today = day(today)
	monday := today.AddDate(0, 0, -((int(today.Weekday()) + 6) % 7))
	first := monday.AddDate(0, 0, -7*(weeks-1))
	out := Dashboard{Today: today.Format(time.DateOnly), Tasks: []Task{}, Projects: []Project{}, Weeks: make([]Week, weeks), Load: []Load{}}
	for i := range out.Weeks {
		out.Weeks[i].Start = workcore.Date{Time: first.AddDate(0, 0, 7*i)}
	}

	rows, err := s.q.DashboardTasks(ctx, sqlc.DashboardTasksParams{UserID: p.UserID, Since: first})
	if err != nil {
		return out, fmt.Errorf("tareas del panel: %w", err)
	}
	type count struct{ open, late int }
	byProject := map[uuid.UUID]*count{}
	names := map[uuid.UUID]string{}
	for _, r := range rows {
		t := Task{
			ID: r.ID, Key: fmt.Sprintf("%s-%d", r.Identifier, r.SequenceNumber), Title: r.Title,
			Project: Ref{ID: r.ProjectID, Name: r.ProjectName}, StatusID: r.StatusID, TypeID: r.TypeID, Closed: r.IsClosed,
			DueDate: workcore.FromTime(r.DueDate), Weight: r.Weight,
			CanMove: p.CanWriteWorkItems(authz.ProjectRef{ID: r.ProjectID, TeamID: r.TeamID, OwnerID: r.OwnerID}),
		}
		names[r.ProjectID] = r.ProjectName
		if !r.IsClosed {
			c := byProject[r.ProjectID]
			if c == nil {
				c = &count{}
				byProject[r.ProjectID] = c
			}
			c.open++
			out.Profile.Open++
			if r.DueDate != nil && day(*r.DueDate).Before(today) {
				t.LateDays = schedule.Days(day(*r.DueDate), today)
				c.late++
				out.Profile.Late++
			}
			out.Tasks = append(out.Tasks, t)
			continue
		}
		if r.ClosedAt == nil {
			continue
		}
		closed := day(*r.ClosedAt)
		t.ClosedOn = &workcore.Date{Time: closed}
		if closed.Year() == today.Year() && closed.Month() == today.Month() {
			out.Profile.ClosedMonth++
		}
		if i := schedule.Days(first, closed) / 7; i >= 0 && i < weeks { // RF-28 to RF-31, RF-36
			w := &out.Weeks[i]
			w.Closed++
			w.Weight += int(r.Weight)
			if r.DueDate != nil {
				w.WithDue++
				if !closed.After(day(*r.DueDate)) {
					w.OnTime++
				}
			}
		}
		if schedule.Days(closed, today) < closedDays { // RF-11
			out.Tasks = append(out.Tasks, t)
		}
	}

	// The load by project, the 8 with the most open work items and «Otros» (RF-33, RF-34).
	ids := make([]uuid.UUID, 0, len(byProject))
	for id := range byProject {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := byProject[ids[i]], byProject[ids[j]]
		if a.open != b.open {
			return a.open > b.open
		}
		return names[ids[i]] < names[ids[j]]
	})
	for i, id := range ids {
		c := byProject[id]
		if i < maxLoad {
			out.Load = append(out.Load, Load{Project: &Ref{ID: id, Name: names[id]}, Open: c.open, Late: c.late})
			continue
		}
		if i == maxLoad {
			out.Load = append(out.Load, Load{})
		}
		out.Load[maxLoad].Open += c.open
		out.Load[maxLoad].Late += c.late
	}

	// The projects the caller works on, the most late first (RF-23 to RF-25).
	teams := make([]uuid.UUID, 0, len(p.ParticipantTeams))
	for id := range p.ParticipantTeams {
		teams = append(teams, id)
	}
	projects, err := s.q.DashboardProjects(ctx, sqlc.DashboardProjectsParams{UserID: p.UserID, TeamIds: teams})
	if err != nil {
		return out, fmt.Errorf("proyectos del panel: %w", err)
	}
	if len(projects) > 0 {
		tree, set, err := portfolio.Load(ctx, s.q, today)
		if err != nil {
			return out, err
		}
		for _, r := range projects {
			ref := authz.ProjectRef{ID: r.ID, TeamID: r.TeamID, OwnerID: r.OwnerID}
			role := string(p.ProjectRoleOf(ref))
			switch {
			case r.OwnerID == p.UserID:
				role = "owner"
			case role == "":
				role = "tasks"
			}
			pr := Project{ID: r.ID, Identifier: r.Identifier, Name: r.Name, Role: role, Figures: tree.Nodes[r.ID].Own(set)}
			if c := byProject[r.ID]; c != nil {
				pr.Open, pr.Late = c.open, c.late
			}
			out.Projects = append(out.Projects, pr)
		}
		sort.SliceStable(out.Projects, func(i, j int) bool { return out.Projects[i].Late > out.Projects[j].Late })
	}
	return out, nil
}

type Handler struct {
	svc    *Service
	logger *slog.Logger
}

func NewHandler(svc *Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

func (h *Handler) Routes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(auth.RequirePermission(authz.WorkRead, h.logger))
		r.Get("/me/dashboard", h.get)
		r.Get("/me/dashboard/people", h.people)
		r.Get("/users/{id}/dashboard", h.get)
		r.Get("/me/team", h.team)
	})
}

// team is Mi equipo at ?today=YYYY-MM-DD, or the server's date (spec 021).
func (h *Handler) team(w http.ResponseWriter, r *http.Request) {
	today, ok := h.today(w, r)
	if !ok {
		return
	}
	out, err := h.svc.Team(r.Context(), authz.FromContext(r.Context()), today)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) today(w http.ResponseWriter, r *http.Request) (time.Time, bool) {
	v := r.URL.Query().Get("today")
	if v == "" {
		return time.Now().UTC(), true
	}
	t, err := time.Parse(time.DateOnly, v)
	if err != nil {
		apperr.Write(w, h.logger, apperr.ErrInvalidFilter.WithDetail("field", "today"))
		return t, false
	}
	return t, true
}

func (h *Handler) people(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.People(r.Context(), authz.FromContext(r.Context()))
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// get reads ?today=YYYY-MM-DD, the caller's date (RF-4), or uses the server's;
// with an {id}, the panel is that person's.
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	p := authz.FromContext(r.Context())
	id := p.UserID
	if v := chi.URLParam(r, "id"); v != "" {
		var err error
		if id, err = uuid.Parse(v); err != nil {
			apperr.Write(w, h.logger, apperr.ErrNotFound)
			return
		}
	}
	today, ok := h.today(w, r)
	if !ok {
		return
	}
	out, err := h.svc.GetFor(r.Context(), p, id, today)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}
