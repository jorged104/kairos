// Package dashboard serves Mi panel: the caller's work items, the projects
// they work on and the series of their progress (spec 013). It only ever
// reads the caller's own data (RF-3).
package dashboard

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"openscum/internal/apperr"
	"openscum/internal/auth"
	"openscum/internal/authz"
	"openscum/internal/db/sqlc"
	"openscum/internal/portfolio"
	"openscum/internal/schedule"
	"openscum/internal/workcore"
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
	ID       uuid.UUID      `json:"id"`
	Key      string         `json:"key"`
	Title    string         `json:"title"`
	Project  Ref            `json:"project"`
	StatusID uuid.UUID      `json:"status_id"`
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

type Dashboard struct {
	Today    string    `json:"today"`
	Profile  Profile   `json:"profile"`
	Tasks    []Task    `json:"tasks"`
	Projects []Project `json:"projects"`
	Weeks    []Week    `json:"weeks"`
	Load     []Load    `json:"load"`
}

type Service struct{ q *sqlc.Queries }

func NewService(q *sqlc.Queries) *Service { return &Service{q: q} }

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
			Project: Ref{ID: r.ProjectID, Name: r.ProjectName}, StatusID: r.StatusID, Closed: r.IsClosed,
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
	r.With(auth.RequirePermission(authz.WorkRead, h.logger)).Get("/me/dashboard", h.get)
}

// get reads ?today=YYYY-MM-DD, the caller's date (RF-4), or uses the server's.
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	today := time.Now().UTC()
	if v := r.URL.Query().Get("today"); v != "" {
		t, err := time.Parse(time.DateOnly, v)
		if err != nil {
			apperr.Write(w, h.logger, apperr.ErrInvalidFilter.WithDetail("field", "today"))
			return
		}
		today = t
	}
	out, err := h.svc.Get(r.Context(), authz.FromContext(r.Context()), today)
	if err != nil {
		apperr.Write(w, h.logger, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}
