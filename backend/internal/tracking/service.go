package tracking

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"

	"openscum/internal/authz"
	"openscum/internal/db/sqlc"
	"openscum/internal/portfolio"
	"openscum/internal/schedule"
	"openscum/internal/workcore"
)

const maxProjects = 50

// Filters of the view; values that do not exist are ignored.
type Filters struct {
	ProgramID  *uuid.UUID
	ProjectIDs []uuid.UUID
	OwnerIDs   []uuid.UUID
}

func (f Filters) empty() bool {
	return f.ProgramID == nil && len(f.ProjectIDs) == 0 && len(f.OwnerIDs) == 0
}

type MilestoneBar struct {
	ID       uuid.UUID        `json:"id"`
	Name     string           `json:"name"`
	DueDate  *workcore.Date   `json:"due_date"`
	Status   string           `json:"status"`
	BarStart *workcore.Date   `json:"bar_start"`
	BarEnd   *workcore.Date   `json:"bar_end"`
	Figures  schedule.Figures `json:"figures"`
	State    string           `json:"state"`
}

type Project struct {
	ID         uuid.UUID        `json:"id"`
	Identifier string           `json:"identifier"`
	Name       string           `json:"name"`
	Owner      workcore.Ref     `json:"owner"`
	Figures    schedule.Figures `json:"figures"`
	Milestones []MilestoneBar   `json:"milestones"`
}

type Summary struct {
	Projects   int              `json:"projects"`
	Milestones int              `json:"milestones"`
	Met        int              `json:"met"`
	Figures    schedule.Figures `json:"figures"`
}

// View is the follow-up of the chosen projects (RF-9 to RF-41).
type View struct {
	Today    string    `json:"today"`
	Matched  int       `json:"matched"` // before the limit of 50 (RF-9)
	Mine     bool      `json:"mine"`    // without filters, the caller's projects (RF-6)
	Projects []Project `json:"projects"`
	Summary  Summary   `json:"summary"`
	Columns  Columns   `json:"columns"`
}

type Service struct{ q *sqlc.Queries }

func NewService(q *sqlc.Queries) *Service { return &Service{q: q} }

// choose applies the filters, all at once, or the caller's projects without
// them (RF-2 to RF-7), always among the projects the caller sees (spec 023,
// RF-21 to RF-23).
func choose(all []sqlc.TrackingProjectsRow, tree portfolio.Tree, p *authz.Principal, f Filters) ([]sqlc.TrackingProjectsRow, bool) {
	var rows []sqlc.TrackingProjectsRow
	for _, r := range all {
		if p.SeesProject(authz.ProjectRef{ID: r.ID, TeamID: r.TeamID, OwnerID: r.OwnerID}) {
			rows = append(rows, r)
		}
	}
	var inProgram map[uuid.UUID]bool
	if f.ProgramID != nil {
		inProgram = map[uuid.UUID]bool{}
		if n, ok := tree.Nodes[*f.ProgramID]; ok && n.IsProgram {
			for _, m := range tree.Members(n.ID) {
				inProgram[m.ID] = true
			}
		}
	}
	has := func(xs []uuid.UUID, x uuid.UUID) bool {
		for _, v := range xs {
			if v == x {
				return true
			}
		}
		return false
	}
	var out []sqlc.TrackingProjectsRow
	if f.empty() {
		for _, r := range rows {
			if p.OwnsTracking(authz.ProjectRef{ID: r.ID, TeamID: r.TeamID, OwnerID: r.OwnerID}) {
				out = append(out, r)
			}
		}
		if len(out) > 0 {
			return out, true
		}
		return rows, false // RF-7
	}
	for _, r := range rows {
		if (inProgram == nil || inProgram[r.ID]) && (len(f.ProjectIDs) == 0 || has(f.ProjectIDs, r.ID)) &&
			(len(f.OwnerIDs) == 0 || has(f.OwnerIDs, r.OwnerID)) {
			out = append(out, r)
		}
	}
	return out, false
}

func (s *Service) Get(ctx context.Context, p *authz.Principal, f Filters, today time.Time) (View, error) {
	out := View{Today: today.Format(time.DateOnly), Projects: []Project{}, Columns: Split(nil, today)}
	tree, set, err := portfolio.Load(ctx, s.q, today)
	if err != nil {
		return out, err
	}
	rows, err := s.q.TrackingProjects(ctx)
	if err != nil {
		return out, fmt.Errorf("proyectos: %w", err)
	}
	chosen, mine := choose(rows, tree, p, f)
	out.Matched, out.Mine = len(chosen), mine
	chosen = chosen[:min(len(chosen), maxProjects)]
	if len(chosen) == 0 {
		return out, nil
	}
	ids := make([]uuid.UUID, len(chosen))
	nodes := make([]portfolio.Node, len(chosen))
	byID := map[uuid.UUID]int{}
	for i, r := range chosen {
		ids[i], nodes[i], byID[r.ID] = r.ID, tree.Nodes[r.ID], i
		out.Projects = append(out.Projects, Project{ID: r.ID, Identifier: r.Identifier, Name: r.Name,
			Owner: workcore.Ref{ID: r.OwnerID, Name: r.OwnerName}, Figures: tree.Nodes[r.ID].Own(set), Milestones: []MilestoneBar{}})
	}
	out.Summary = Summary{Projects: len(chosen), Figures: portfolio.Combine(nodes, set)} // RF-11, RF-13

	if err := s.milestones(ctx, ids, byID, tree, set, today, &out); err != nil {
		return out, err
	}
	leaves, err := s.q.TrackingLeafItems(ctx, ids)
	if err != nil {
		return out, fmt.Errorf("tareas: %w", err)
	}
	items := make([]LeafItem, 0, len(leaves))
	for _, l := range leaves {
		it := LeafItem{Task: Task{ID: l.ID, Key: l.Identifier + "-" + strconv.Itoa(int(l.SequenceNumber)), Title: l.Title, ProjectID: l.ProjectID,
			ParentTitle: l.ParentTitle, Ratio: l.Ratio, StartDate: workcore.FromTime(l.StartDate), DueDate: workcore.FromTime(l.DueDate)},
			Start: l.StartDate, Due: l.DueDate, IsClosed: l.IsClosed, Initial: l.IsDefault}
		if l.ClosedAt != nil {
			c := l.ClosedAt.UTC()
			it.Closed, it.ClosedOn = &c, workcore.FromTime(&c)
		}
		items = append(items, it)
	}
	out.Columns = Split(items, today)
	return out, nil
}

// milestones builds the calendar of the chosen projects (RF-12, RF-15 to RF-28).
func (s *Service) milestones(ctx context.Context, ids []uuid.UUID, byID map[uuid.UUID]int, tree portfolio.Tree, set schedule.Settings,
	today time.Time, out *View) error {
	ms, err := s.q.TrackingMilestones(ctx, ids)
	if err != nil {
		return fmt.Errorf("hitos: %w", err)
	}
	pairs, err := s.q.TrackingMilestonePairs(ctx, ids)
	if err != nil {
		return fmt.Errorf("tareas de los hitos: %w", err)
	}
	items := map[uuid.UUID]schedule.Item{}
	for _, id := range ids {
		for _, it := range tree.Items[id] {
			items[it.ID] = it
		}
	}
	members := map[uuid.UUID][]schedule.Item{}
	for _, p := range pairs {
		if it, ok := items[p.WorkItemID]; ok {
			members[p.MilestoneID] = append(members[p.MilestoneID], it)
		}
	}
	for _, m := range ms {
		its := members[m.ID]
		plan, ok := schedule.Plan(its, today)
		f := schedule.Make(plan, ok && m.Status != statusMet, MilestoneReal(its), set)
		start, end := Bar(its)
		bar := MilestoneBar{ID: m.ID, Name: m.Name, DueDate: workcore.FromTime(m.DueDate), Status: m.Status,
			BarStart: workcore.FromTime(start), BarEnd: workcore.FromTime(end), Figures: f}
		bar.State = State(m.Status, m.DueDate, start, schedule.Figures{HasPlan: ok, Level: f.Level}, today)
		p := &out.Projects[byID[m.ProjectID]]
		p.Milestones = append(p.Milestones, bar)
		out.Summary.Milestones++
		if m.Status == statusMet {
			out.Summary.Met++
		}
	}
	return nil
}
