package workitems

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	"openscum/internal/apperr"
	"openscum/internal/workcore"
)

// Filters of the work items of a project, shared by the list, the tree and the
// saved views (spec 004, RF-129; spec 008, RF-18 to RF-20). Different
// parameters combine with AND; the values of a repeated one with OR.
//
//   - q: text in the title, or the number (KAI-12 or 12)
//   - status_id, type_id, priority_id, size_id (or "none"), label_id,
//     milestone_id (effective), assignee_id (or "none", "me")
//   - closed, blocked: true or false
//   - due_before, due_after: dates, both included; overdue=true: open and
//     due before today (today, or the server's date in UTC)
var FilterParams = []string{"q", "status_id", "type_id", "priority_id", "size_id", "label_id", "milestone_id",
	"assignee_id", "closed", "blocked", "due_before", "due_after", "overdue"}

// Filtering reports whether a query has any filter.
func Filtering(query url.Values) bool {
	for _, k := range FilterParams {
		for _, v := range query[k] {
			if strings.TrimSpace(v) != "" {
				return true
			}
		}
	}
	return false
}

// Where builds the conditions over "work_items w" joined with "statuses s".
func Where(projectID uuid.UUID, identifier string, me uuid.UUID, query url.Values) (sq.And, error) {
	where := sq.And{sq.Eq{"w.project_id": projectID}}
	bad := func(field string) error { return apperr.ErrInvalidFilter.WithDetail("field", field) }
	values := func(param string) []string {
		var out []string
		for _, v := range query[param] {
			if v = strings.TrimSpace(v); v != "" {
				out = append(out, v)
			}
		}
		return out
	}
	ids := func(param string, special ...string) ([]uuid.UUID, []string, error) {
		var out []uuid.UUID
		var named []string
		for _, v := range values(param) {
			if slicesContains(special, v) {
				named = append(named, v)
				continue
			}
			id, err := uuid.Parse(v)
			if err != nil {
				return nil, nil, bad(param)
			}
			out = append(out, id)
		}
		return out, named, nil
	}

	if text := strings.TrimSpace(query.Get("q")); text != "" {
		cond := sq.Or{sq.Expr("unaccent(lower(w.title)) LIKE unaccent(lower(?))", "%"+workcore.EscapeLike(text)+"%")}
		num := strings.TrimPrefix(strings.ToUpper(text), identifier+"-")
		if n, err := strconv.Atoi(num); err == nil {
			cond = append(cond, sq.Eq{"w.sequence_number": n})
		}
		where = append(where, cond)
	}
	for param, col := range map[string]string{"type_id": "w.type_id", "status_id": "w.status_id", "priority_id": "w.priority_id"} {
		list, _, err := ids(param)
		if err != nil {
			return nil, err
		}
		if len(list) > 0 {
			where = append(where, sq.Eq{col: list})
		}
	}
	sizes, noSize, err := ids("size_id", "none")
	if err != nil {
		return nil, err
	}
	if len(sizes) > 0 || len(noSize) > 0 {
		cond := sq.Or{}
		if len(sizes) > 0 {
			cond = append(cond, sq.Eq{"w.size_id": sizes})
		}
		if len(noSize) > 0 {
			cond = append(cond, sq.Eq{"w.size_id": nil})
		}
		where = append(where, cond)
	}
	// A work item matches by any of its assignees (spec 006, RF-19); "me" is
	// whoever looks at the view (spec 008, RF-19).
	people, named, err := ids("assignee_id", "none", "me")
	if err != nil {
		return nil, err
	}
	if len(people) > 0 || len(named) > 0 {
		cond := sq.Or{}
		for _, n := range named {
			if n == "none" {
				cond = append(cond, sq.Expr("NOT EXISTS (SELECT 1 FROM work_item_assignees a WHERE a.work_item_id = w.id)"))
			} else {
				people = append(people, me)
			}
		}
		if len(people) > 0 {
			cond = append(cond, sq.Expr("EXISTS (SELECT 1 FROM work_item_assignees a WHERE a.work_item_id = w.id AND a.user_id = ANY (?))", people))
		}
		where = append(where, cond)
	}
	// Any of the labels (spec 007, RF-44).
	labels, _, err := ids("label_id")
	if err != nil {
		return nil, err
	}
	if len(labels) > 0 {
		where = append(where, sq.Expr("EXISTS (SELECT 1 FROM work_item_labels l WHERE l.work_item_id = w.id AND l.label_id = ANY (?))", labels))
	}
	milestones, _, err := ids("milestone_id")
	if err != nil {
		return nil, err
	}
	if len(milestones) > 0 {
		where = append(where, sq.Expr(`w.id IN (
			WITH RECURSIVE eff AS (
				SELECT mw.work_item_id AS id FROM milestone_work_items mw WHERE mw.milestone_id = ANY (?)
				UNION
				SELECT c.id FROM work_items c JOIN eff ON c.parent_id = eff.id
			) SELECT id FROM eff)`, milestones))
	}
	flag := func(param string) (*bool, error) {
		switch v := query.Get(param); v {
		case "":
			return nil, nil
		case "true", "false":
			b := v == "true"
			return &b, nil
		default:
			return nil, bad(param)
		}
	}
	closed, err := flag("closed")
	if err != nil {
		return nil, err
	}
	if closed != nil {
		where = append(where, sq.Eq{"s.is_closed": *closed})
	}
	blocked, err := flag("blocked")
	if err != nil {
		return nil, err
	}
	if blocked != nil {
		exists := `EXISTS (SELECT 1 FROM work_item_dependencies d JOIN work_items b ON b.id = d.from_id
			JOIN statuses bs ON bs.id = b.status_id WHERE d.to_id = w.id AND d.type = 'blocks' AND NOT bs.is_closed)`
		if *blocked {
			where = append(where, sq.Expr(exists))
		} else {
			where = append(where, sq.Expr("NOT "+exists))
		}
	}
	date := func(param string) (*time.Time, error) {
		v := query.Get(param)
		if v == "" {
			return nil, nil
		}
		t, err := time.Parse(time.DateOnly, v)
		if err != nil {
			return nil, bad(param)
		}
		return &t, nil
	}
	before, err := date("due_before")
	if err != nil {
		return nil, err
	}
	if before != nil {
		where = append(where, sq.LtOrEq{"w.due_date": *before})
	}
	after, err := date("due_after")
	if err != nil {
		return nil, err
	}
	if after != nil {
		where = append(where, sq.GtOrEq{"w.due_date": *after})
	}
	overdue, err := flag("overdue")
	if err != nil {
		return nil, err
	}
	if overdue != nil && *overdue {
		today, err := date("today")
		if err != nil {
			return nil, err
		}
		if today == nil {
			now := time.Now().UTC()
			t := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
			today = &t
		}
		where = append(where, sq.Lt{"w.due_date": *today}, sq.Eq{"s.is_closed": false})
	}
	return where, nil
}

// sortColumns are the orders of the list (spec 008, RF-57); a leading "-"
// sorts descending. Statuses and priorities go by their catalog position.
var sortColumns = map[string]string{
	"key": "w.sequence_number", "title": "lower(w.title)", "status": "s.position", "due": "w.due_date",
	"progress": "w.weighted_done_ratio", "weight": "w.weight",
	"priority": "(SELECT pr.position FROM priorities pr WHERE pr.id = w.priority_id)",
}

// OrderBy is the ORDER BY of a sort parameter; empty is the default order.
func OrderBy(query url.Values, fallback string) (string, error) {
	v := query.Get("sort")
	if v == "" {
		return fallback, nil
	}
	dir := "ASC"
	if strings.HasPrefix(v, "-") {
		dir, v = "DESC", v[1:]
	}
	col, ok := sortColumns[v]
	if !ok {
		return "", apperr.ErrInvalidFilter.WithDetail("field", "sort")
	}
	return col + " " + dir + " NULLS LAST, w.sequence_number", nil
}

func slicesContains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
