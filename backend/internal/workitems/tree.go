package workitems

import (
	"context"
	"fmt"
	"net/url"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	"openscum/internal/apperr"
	"openscum/internal/authz"
	"openscum/internal/db/sqlc"
	"openscum/internal/projects"
	"openscum/internal/workcore"
	"openscum/pkg/httpx"
)

const (
	DefaultPageSize = 50
	MaxPageSize     = 200
)

type Page struct {
	Items    []Node `json:"items"`
	Total    int64  `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

// Tree returns every work item of a project as a flat list ordered by depth;
// the client builds the tree (RF-130). With filters, only the matching ones,
// for the kanban, the timeline and the filtered table (spec 008, RF-22).
func (s *Service) Tree(ctx context.Context, p *authz.Principal, projectID uuid.UUID, query url.Values) ([]Node, error) {
	project, err := projects.Get(ctx, s.store.Queries, p, projectID, false)
	if err != nil {
		return nil, err
	}
	where, err := Where(projectID, project.Identifier, p.UserID, query)
	if err != nil {
		return nil, err
	}
	order, err := OrderBy(query, "w.depth, w.position, w.sequence_number")
	if err != nil {
		return nil, err
	}
	return s.nodes(ctx, project, psql.Select().From("work_items w").Join("statuses s ON s.id = w.status_id").Where(where).OrderBy(order))
}

// Descendants returns every descendant of a work item in one query (RF-79).
func (s *Service) Descendants(ctx context.Context, p *authz.Principal, id uuid.UUID) ([]Node, error) {
	_, project, err := Load(ctx, s.store.Queries, p, id, false)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.WorkItemDescendants(ctx, &id)
	if err != nil {
		return nil, fmt.Errorf("leer descendientes: %w", err)
	}
	out := make([]Node, 0, len(rows))
	for _, r := range rows {
		out = append(out, NodeFrom(project.Identifier, r))
	}
	return out, fillAll(ctx, s.store.Queries, out)
}

// NodeFrom builds a tree row; identifier is the project identifier.
func NodeFrom(identifier string, r sqlc.WorkItemDescendantsRow) Node {
	n := Node{
		ID: r.ID, ParentID: r.ParentID, Key: Key(identifier, r.SequenceNumber), Title: r.Title, TypeID: r.TypeID,
		StatusID: r.StatusID, PriorityID: r.PriorityID, Estimated: r.AggEstimated, Remaining: r.AggRemaining,
		Depth: r.Depth, Position: r.Position, DueDate: workcore.FromTime(r.DueDate), Weight: r.Weight,
		SizeID: r.SizeID, Assignees: []workcore.Ref{},
	}
	if r.DerivedDoneRatio != nil {
		n.DoneRatio = *r.DerivedDoneRatio
	}
	if r.AssigneeID != nil && r.AssigneeName != nil {
		n.Assignee = &workcore.Ref{ID: *r.AssigneeID, Name: *r.AssigneeName}
	}
	return n
}

// FillLists sets the assignees and the labels of some rows, one query each
// (spec 006, RF-12, RF-18; spec 007, RF-43). Assignee stays as the first
// assignee, for the previous clients.
func FillLists(ctx context.Context, q *sqlc.Queries, nodes []*Node) error {
	if len(nodes) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(nodes))
	for i, n := range nodes {
		ids[i] = n.ID
	}
	byItem, err := assigneesOf(ctx, q, ids)
	if err != nil {
		return err
	}
	labels, err := labelsOf(ctx, q, ids)
	if err != nil {
		return err
	}
	for _, n := range nodes {
		n.LabelIDs = labels[n.ID]
		if n.LabelIDs == nil {
			n.LabelIDs = []uuid.UUID{}
		}
		n.Assignees = byItem[n.ID]
		if n.Assignees == nil {
			n.Assignees = []workcore.Ref{}
		}
		n.Assignee = nil
		if len(n.Assignees) > 0 {
			first := n.Assignees[0]
			n.Assignee = &first
		}
	}
	return nil
}

func fillAll(ctx context.Context, q *sqlc.Queries, nodes []Node) error {
	ptrs := make([]*Node, len(nodes))
	for i := range nodes {
		ptrs[i] = &nodes[i]
	}
	return FillLists(ctx, q, ptrs)
}

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

// List returns the work items of a project, a page at a time (RF-129), with
// the filters and orders of filters.go (spec 008).
func (s *Service) List(ctx context.Context, p *authz.Principal, projectID uuid.UUID, query url.Values) (Page, error) {
	project, err := projects.Get(ctx, s.store.Queries, p, projectID, false)
	if err != nil {
		return Page{}, err
	}
	page, ok := httpx.ParsePage(query, DefaultPageSize, MaxPageSize)
	if !ok {
		return Page{}, apperr.ErrInvalidFilter.WithDetail("field", "page")
	}
	where, err := Where(projectID, project.Identifier, p.UserID, query)
	if err != nil {
		return Page{}, err
	}
	order, err := OrderBy(query, "w.sequence_number")
	if err != nil {
		return Page{}, err
	}
	base := psql.Select().From("work_items w").Join("statuses s ON s.id = w.status_id").Where(where)
	countSQL, countArgs, err := base.Column("count(*)").ToSql()
	if err != nil {
		return Page{}, fmt.Errorf("construir conteo: %w", err)
	}
	var total int64
	if err := s.store.Pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return Page{}, fmt.Errorf("contar work items: %w", err)
	}
	items, err := s.nodes(ctx, project, base.OrderBy(order).Limit(uint64(page.Size)).Offset(uint64(page.Offset())))
	if err != nil {
		return Page{}, err
	}
	return Page{Items: items, Total: total, Page: page.Number, PageSize: page.Size}, nil
}

// Count is how many work items of a project match some filters (spec 008, RF-26).
func (s *Service) Count(ctx context.Context, p *authz.Principal, project sqlc.Project, query url.Values) (int64, error) {
	where, err := Where(project.ID, project.Identifier, p.UserID, query)
	if err != nil {
		return 0, err
	}
	sql, args, err := psql.Select("count(*)").From("work_items w").Join("statuses s ON s.id = w.status_id").Where(where).ToSql()
	if err != nil {
		return 0, fmt.Errorf("construir conteo: %w", err)
	}
	var n int64
	if err := s.store.Pool.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("contar work items: %w", err)
	}
	return n, nil
}

// nodes runs a select of work items and fills their lists, whether they are
// blocked and their effective milestones (spec 008, RF-61, RF-70).
func (s *Service) nodes(ctx context.Context, project sqlc.Project, sel sq.SelectBuilder) ([]Node, error) {
	sql, args, err := sel.Columns(
		"w.id", "w.parent_id", "w.sequence_number", "w.title", "w.type_id", "w.status_id", "w.priority_id",
		"w.assignee_id", "u.name", "w.weighted_done_ratio", "w.agg_estimated", "w.agg_remaining",
		"w.depth", "w.position", "w.due_date", "w.weight", "w.size_id", "w.start_date", "w.closed_at",
	).LeftJoin("users u ON u.id = w.assignee_id").ToSql()
	if err != nil {
		return nil, fmt.Errorf("construir listado: %w", err)
	}
	rows, err := s.store.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("listar work items: %w", err)
	}
	defer rows.Close()
	out := []Node{}
	for rows.Next() {
		var r sqlc.WorkItemDescendantsRow
		var due, start, closed *time.Time
		if err := rows.Scan(&r.ID, &r.ParentID, &r.SequenceNumber, &r.Title, &r.TypeID, &r.StatusID, &r.PriorityID,
			&r.AssigneeID, &r.AssigneeName, &r.DerivedDoneRatio, &r.AggEstimated, &r.AggRemaining, &r.Depth, &r.Position,
			&due, &r.Weight, &r.SizeID, &start, &closed); err != nil {
			return nil, fmt.Errorf("leer work item: %w", err)
		}
		r.DueDate = due
		n := NodeFrom(project.Identifier, r)
		n.StartDate, n.ClosedAt = workcore.FromTime(start), closed
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if err := fillAll(ctx, s.store.Queries, out); err != nil {
		return nil, err
	}
	return out, s.fillPlanning(ctx, project.ID, out)
}

func (s *Service) fillPlanning(ctx context.Context, projectID uuid.UUID, nodes []Node) error {
	blocked, err := s.store.ProjectBlockedItems(ctx, projectID)
	if err != nil {
		return fmt.Errorf("leer bloqueos: %w", err)
	}
	isBlocked := map[uuid.UUID]bool{}
	for _, id := range blocked {
		isBlocked[id] = true
	}
	eff, err := s.store.ProjectEffectiveMilestones(ctx, projectID)
	if err != nil {
		return fmt.Errorf("leer hitos efectivos: %w", err)
	}
	milestones := map[uuid.UUID][]uuid.UUID{}
	for _, e := range eff {
		milestones[e.ID] = append(milestones[e.ID], e.MilestoneID)
	}
	deps, err := s.store.ProjectTimelineDependencies(ctx, projectID)
	if err != nil {
		return fmt.Errorf("leer dependencias: %w", err)
	}
	successors := map[uuid.UUID][]Successor{}
	for _, d := range deps {
		successors[d.FromID] = append(successors[d.FromID], Successor{ToID: d.ToID, Type: d.Type, LagDays: d.LagDays})
	}
	for i := range nodes {
		nodes[i].Blocked = isBlocked[nodes[i].ID]
		nodes[i].MilestoneIDs = milestones[nodes[i].ID]
		nodes[i].Successors = successors[nodes[i].ID]
		if nodes[i].MilestoneIDs == nil {
			nodes[i].MilestoneIDs = []uuid.UUID{}
		}
	}
	return nil
}
