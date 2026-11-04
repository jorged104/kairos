// Package projectboard serves the project list in its three modes, Kanban,
// Timeline and Table, and its saved views (spec 012). Filters, order and
// grouping run in the browser over the whole board, so «Yo», the traffic
// light and the count of each view share one calculation (RF-62 to RF-72).
package projectboard

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"openscum/internal/authz"
	"openscum/internal/db/sqlc"
	"openscum/internal/portfolio"
	"openscum/internal/schedule"
	"openscum/internal/workcore"
)

type Ref struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type Milestone struct {
	ID      uuid.UUID     `json:"id"`
	Name    string        `json:"name"`
	DueDate workcore.Date `json:"due_date"`
	Done    bool          `json:"done"`
}

// Card is a project as the three modes draw it.
type Card struct {
	ID         uuid.UUID      `json:"id"`
	Identifier string         `json:"identifier"`
	Name       string         `json:"name"`
	StatusID   uuid.UUID      `json:"status_id"`
	Lifecycle  string         `json:"lifecycle"`
	Health     string         `json:"health"`
	Team       Ref            `json:"team"`
	Owner      Ref            `json:"owner"`
	IsProgram  bool           `json:"is_program"`
	Program    *Ref           `json:"program"` // the nearest program above it
	StartDate  *workcore.Date `json:"start_date"`
	EndDate    *workcore.Date `json:"end_date"`
	BaseStart  *workcore.Date `json:"base_start"` // of its last baseline (RF-49)
	BaseEnd    *workcore.Date `json:"base_end"`
	Version    int32          `json:"version"`
	CanManage  bool           `json:"can_manage"`
	// A program's are its aggregate (spec 010); a project's, its own.
	Figures    schedule.Figures `json:"figures"`
	Milestones []Milestone      `json:"milestones"`
}

// Program is a program not archived with its aggregate, for the groups of
// the timeline (RF-52).
type Program struct {
	ID      uuid.UUID        `json:"id"`
	Name    string           `json:"name"`
	Figures schedule.Figures `json:"figures"`
}

type Board struct {
	Today    string    `json:"today"`
	Projects []Card    `json:"projects"`
	Programs []Program `json:"programs"`
}

type Service struct{ q *sqlc.Queries }

func NewService(q *sqlc.Queries) *Service { return &Service{q: q} }

// Get returns every project not archived that is not a template (RF-29).
func (s *Service) Get(ctx context.Context, p *authz.Principal, today time.Time) (Board, error) {
	out := Board{Today: today.Format(time.DateOnly), Projects: []Card{}, Programs: []Program{}}
	tree, set, err := portfolio.Load(ctx, s.q, today)
	if err != nil {
		return out, err
	}
	rows, err := s.q.BoardProjects(ctx)
	if err != nil {
		return out, fmt.Errorf("proyectos del tablero: %w", err)
	}
	ms, err := s.q.BoardMilestones(ctx)
	if err != nil {
		return out, fmt.Errorf("hitos del tablero: %w", err)
	}
	byProject := map[uuid.UUID][]Milestone{}
	for _, m := range ms {
		byProject[m.ProjectID] = append(byProject[m.ProjectID], Milestone{ID: m.ID, Name: m.Name, DueDate: workcore.Date{Time: *m.DueDate}, Done: m.Status == "cumplido"})
	}
	bases, err := s.q.BoardBaselineDates(ctx)
	if err != nil {
		return out, fmt.Errorf("líneas base del tablero: %w", err)
	}
	type span struct{ from, to *time.Time }
	base := map[uuid.UUID]span{}
	for _, b := range bases {
		sp := base[b.ProjectID]
		start := b.StartDate
		if start == nil {
			start = b.DueDate
		}
		if start != nil && (sp.from == nil || start.Before(*sp.from)) {
			sp.from = start
		}
		if b.DueDate != nil && (sp.to == nil || b.DueDate.After(*sp.to)) {
			sp.to = b.DueDate
		}
		base[b.ProjectID] = sp
	}

	figures := func(id uuid.UUID) schedule.Figures {
		n := tree.Nodes[id]
		if n.IsProgram {
			return tree.Aggregate(id, set)
		}
		return n.Own(set)
	}
	for _, r := range rows {
		c := Card{
			ID: r.ID, Identifier: r.Identifier, Name: r.Name, StatusID: r.StatusID, Lifecycle: r.Lifecycle, Health: r.Health,
			Team: Ref{ID: r.TeamID, Name: r.TeamName}, Owner: Ref{ID: r.OwnerID, Name: r.OwnerName}, IsProgram: r.IsProgram,
			StartDate: workcore.FromTime(r.StartDate), EndDate: workcore.FromTime(r.EndDate),
			BaseStart: workcore.FromTime(base[r.ID].from), BaseEnd: workcore.FromTime(base[r.ID].to), Version: r.Version,
			CanManage:  p.CanManageProject(authz.ProjectRef{ID: r.ID, TeamID: r.TeamID, OwnerID: r.OwnerID}),
			Figures:    figures(r.ID),
			Milestones: byProject[r.ID],
		}
		if c.Milestones == nil {
			c.Milestones = []Milestone{}
		}
		for up := r.ParentID; up != nil; up = tree.Nodes[*up].ParentID {
			n, ok := tree.Nodes[*up]
			if !ok {
				break
			}
			if n.IsProgram && n.Lifecycle != "archivado" {
				c.Program = &Ref{ID: n.ID, Name: n.Name}
				break
			}
		}
		if r.IsProgram {
			out.Programs = append(out.Programs, Program{ID: r.ID, Name: r.Name, Figures: c.Figures})
		}
		out.Projects = append(out.Projects, c)
	}
	return out, nil
}
