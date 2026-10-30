package workitems

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
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
// the client builds the tree (RF-130).
func (s *Service) Tree(ctx context.Context, p *authz.Principal, projectID uuid.UUID) ([]Node, error) {
	project, err := projects.Get(ctx, s.store.Queries, p, projectID, false)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.ProjectWorkItemTree(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("leer árbol: %w", err)
	}
	out := make([]Node, 0, len(rows))
	for _, r := range rows {
		out = append(out, NodeFrom(project.Identifier, sqlc.WorkItemDescendantsRow(r)))
	}
	return out, fillAll(ctx, s.store.Queries, out)
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

// FillAssignees sets the assignees of some rows in one query (RF-12, RF-18).
// Assignee stays as the first of them, for the previous clients.
func FillAssignees(ctx context.Context, q *sqlc.Queries, nodes []*Node) error {
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
	for _, n := range nodes {
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
	return FillAssignees(ctx, q, ptrs)
}

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

// List returns the work items of a project by number (RF-129). Query
// parameters: q (title or number), type_id, status_id, closed (true/false),
// priority_id, assignee_id (or "none"), milestone_id (effective, inherited
// included), page, page_size.
func (s *Service) List(ctx context.Context, p *authz.Principal, projectID uuid.UUID, query url.Values) (Page, error) {
	project, err := projects.Get(ctx, s.store.Queries, p, projectID, false)
	if err != nil {
		return Page{}, err
	}
	page, ok := httpx.ParsePage(query, DefaultPageSize, MaxPageSize)
	if !ok {
		return Page{}, apperr.ErrInvalidFilter.WithDetail("field", "page")
	}
	where := sq.And{sq.Eq{"w.project_id": projectID}}
	if text := strings.TrimSpace(query.Get("q")); text != "" {
		cond := sq.Or{sq.Expr("unaccent(lower(w.title)) LIKE unaccent(lower(?))", "%"+workcore.EscapeLike(text)+"%")}
		num := strings.TrimPrefix(strings.ToUpper(text), project.Identifier+"-")
		if n, err := strconv.Atoi(num); err == nil {
			cond = append(cond, sq.Eq{"w.sequence_number": n})
		}
		where = append(where, cond)
	}
	for param, col := range map[string]string{"type_id": "w.type_id", "status_id": "w.status_id", "priority_id": "w.priority_id"} {
		if v := query.Get(param); v != "" {
			id, err := uuid.Parse(v)
			if err != nil {
				return Page{}, apperr.ErrInvalidFilter.WithDetail("field", param)
			}
			where = append(where, sq.Eq{col: id})
		}
	}
	// A work item matches by any of its assignees (RF-19).
	switch v := query.Get("assignee_id"); v {
	case "":
	case "none":
		where = append(where, sq.Expr("NOT EXISTS (SELECT 1 FROM work_item_assignees a WHERE a.work_item_id = w.id)"))
	default:
		id, err := uuid.Parse(v)
		if err != nil {
			return Page{}, apperr.ErrInvalidFilter.WithDetail("field", "assignee_id")
		}
		where = append(where, sq.Expr("EXISTS (SELECT 1 FROM work_item_assignees a WHERE a.work_item_id = w.id AND a.user_id = ?)", id))
	}
	switch v := query.Get("closed"); v {
	case "":
	case "true", "false":
		where = append(where, sq.Eq{"s.is_closed": v == "true"})
	default:
		return Page{}, apperr.ErrInvalidFilter.WithDetail("field", "closed")
	}
	if v := query.Get("milestone_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return Page{}, apperr.ErrInvalidFilter.WithDetail("field", "milestone_id")
		}
		where = append(where, sq.Expr(`w.id IN (
			WITH RECURSIVE eff AS (
				SELECT mw.work_item_id AS id FROM milestone_work_items mw WHERE mw.milestone_id = ?
				UNION
				SELECT c.id FROM work_items c JOIN eff ON c.parent_id = eff.id
			) SELECT id FROM eff)`, id))
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
	listSQL, args, err := base.Columns(
		"w.id", "w.parent_id", "w.sequence_number", "w.title", "w.type_id", "w.status_id", "w.priority_id",
		"w.assignee_id", "u.name", "w.weighted_done_ratio", "w.agg_estimated", "w.agg_remaining",
		"w.depth", "w.position", "w.due_date", "w.weight", "w.size_id",
	).LeftJoin("users u ON u.id = w.assignee_id").OrderBy("w.sequence_number").
		Limit(uint64(page.Size)).Offset(uint64(page.Offset())).ToSql()
	if err != nil {
		return Page{}, fmt.Errorf("construir listado: %w", err)
	}
	rows, err := s.store.Pool.Query(ctx, listSQL, args...)
	if err != nil {
		return Page{}, fmt.Errorf("listar work items: %w", err)
	}
	defer rows.Close()
	out := Page{Items: []Node{}, Total: total, Page: page.Number, PageSize: page.Size}
	for rows.Next() {
		var r sqlc.WorkItemDescendantsRow
		var due *time.Time
		if err := rows.Scan(&r.ID, &r.ParentID, &r.SequenceNumber, &r.Title, &r.TypeID, &r.StatusID, &r.PriorityID,
			&r.AssigneeID, &r.AssigneeName, &r.DerivedDoneRatio, &r.AggEstimated, &r.AggRemaining, &r.Depth, &r.Position, &due, &r.Weight, &r.SizeID); err != nil {
			return Page{}, fmt.Errorf("leer work item: %w", err)
		}
		r.DueDate = due
		out.Items = append(out.Items, NodeFrom(project.Identifier, r))
	}
	if err := rows.Err(); err != nil {
		return Page{}, err
	}
	rows.Close()
	return out, fillAll(ctx, s.store.Queries, out.Items)
}
