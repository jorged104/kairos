package portfolio

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"openscum/internal/db/sqlc"
	"openscum/internal/schedule"
)

// Load builds the tree of the portfolio with each project's own plan and
// real progress at today's date.
func Load(ctx context.Context, q *sqlc.Queries, today time.Time) (Tree, schedule.Settings, error) {
	set, err := schedule.Load(ctx, q)
	if err != nil {
		return Tree{}, set, err
	}
	projects, err := q.PortfolioProjects(ctx)
	if err != nil {
		return Tree{}, set, fmt.Errorf("proyectos del portafolio: %w", err)
	}
	rows, err := q.PortfolioPlanItems(ctx)
	if err != nil {
		return Tree{}, set, fmt.Errorf("tareas del portafolio: %w", err)
	}
	items := map[uuid.UUID][]schedule.Item{}
	for _, r := range rows {
		items[r.ProjectID] = append(items[r.ProjectID], schedule.FromRow(sqlc.ProjectPlanItemsRow{
			ID: r.ID, ParentID: r.ParentID, Weight: r.Weight, IsClosed: r.IsClosed, Ratio: r.Ratio,
			StartDate: r.StartDate, DueDate: r.DueDate, InBaseline: r.InBaseline, BaseStart: r.BaseStart, BaseDue: r.BaseDue,
		}))
	}
	nodes := make([]Node, 0, len(projects))
	for _, p := range projects {
		its := items[p.ID]
		plan, ok := schedule.ProjectValue(p.StartDate, p.EndDate, its, today)
		nodes = append(nodes, Node{ID: p.ID, ParentID: p.ParentID, Identifier: p.Identifier, Name: p.Name, Lifecycle: p.Lifecycle,
			IsProgram: p.IsProgram, ManualWeight: p.PortfolioWeight, Effort: p.Effort, Plan: plan, HasPlan: ok, Real: schedule.Real(its)})
	}
	tree := NewTree(nodes)
	tree.Items = items
	return tree, set, nil
}

// Portfolio is every program and the projects without one (RF-22 to RF-29).
type Portfolio struct {
	Today    string `json:"today"`
	Programs []Card `json:"programs"`
	Loose    Card   `json:"loose"`
}

type Service struct{ q *sqlc.Queries }

func NewService(q *sqlc.Queries) *Service { return &Service{q: q} }

func (s *Service) Get(ctx context.Context, today time.Time) (Portfolio, error) {
	tree, set, err := Load(ctx, s.q, today)
	if err != nil {
		return Portfolio{}, err
	}
	programs, loose := tree.Cards(set)
	return Portfolio{Today: today.Format(time.DateOnly), Programs: programs, Loose: loose}, nil
}
