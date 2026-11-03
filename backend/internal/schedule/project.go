package schedule

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"openscum/internal/db/sqlc"
)

// ProjectPlan is everything the plan of a project needs at today's date.
type ProjectPlan struct {
	Settings Settings
	Rows     []sqlc.ProjectPlanItemsRow
	Items    []Item
	// Plan of the project's work items; false when none has a planned end.
	Plan    float64
	HasPlan bool
	Real    float64
	// Per work item, over its subtree (RF-19).
	ByItem map[uuid.UUID]Figures
	// Plan of each milestone over its effective work items (RF-21).
	Milestones map[uuid.UUID]MilestonePlan
}

type MilestonePlan struct {
	Plan    float64
	HasPlan bool
}

// LoadProject computes the plan of a project, its work items and its
// milestones.
func LoadProject(ctx context.Context, q *sqlc.Queries, projectID uuid.UUID, today time.Time) (ProjectPlan, error) {
	set, err := Load(ctx, q)
	if err != nil {
		return ProjectPlan{}, err
	}
	rows, err := q.ProjectPlanItems(ctx, projectID)
	if err != nil {
		return ProjectPlan{}, fmt.Errorf("tareas del plan: %w", err)
	}
	out := ProjectPlan{Settings: set, Rows: rows, Items: make([]Item, len(rows)), Milestones: map[uuid.UUID]MilestonePlan{}}
	byID := make(map[uuid.UUID]Item, len(rows))
	for i, r := range rows {
		out.Items[i] = FromRow(r)
		byID[r.ID] = out.Items[i]
	}
	out.Plan, out.HasPlan = Plan(out.Items, today)
	out.Real = Real(out.Items)
	out.ByItem = SubtreePlans(out.Items, today, set)

	pairs, err := q.ProjectMilestoneItemPairs(ctx, projectID)
	if err != nil {
		return out, fmt.Errorf("tareas de los hitos: %w", err)
	}
	members := map[uuid.UUID][]Item{}
	for _, p := range pairs {
		if it, ok := byID[p.WorkItemID]; ok {
			members[p.MilestoneID] = append(members[p.MilestoneID], it)
		}
	}
	for m, items := range members {
		plan, ok := Plan(items, today)
		out.Milestones[m] = MilestonePlan{Plan: plan, HasPlan: ok}
	}
	return out, nil
}
