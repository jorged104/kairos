package users

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	"openscum/internal/apperr"
	"openscum/internal/attributes"
	"openscum/internal/authz"
	"openscum/pkg/httpx"
)

const (
	DefaultPageSize = 25
	MaxPageSize     = 100
)

type DirectoryPage struct {
	Items    []User `json:"items"`
	Total    int64  `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

// Directory lists users sorted by name with search, filters and paging
// (RF-109 to RF-116). Query parameters:
//
//	q, team_id, role, status, page, page_size,
//	attr.<id>=value (text, boolean, select; also exact number/date),
//	attr.<id>.from / attr.<id>.to (number and date ranges).
//
// Fields outside the viewer's full-profile scope (email, roles, attributes)
// are neither returned nor usable to search or filter.
func (s *Service) Directory(ctx context.Context, p *authz.Principal, query url.Values) (DirectoryPage, error) {
	page, ok := httpx.ParsePage(query, DefaultPageSize, MaxPageSize)
	if !ok {
		return DirectoryPage{}, apperr.ErrInvalidFilter.WithDetail("field", "page")
	}
	visible := s.visibleFull(p)
	where := sq.And{}

	if text := strings.TrimSpace(query.Get("q")); text != "" {
		like := "%" + escapeLike(text) + "%"
		where = append(where, sq.Or{
			sq.Expr("unaccent(lower(u.name)) LIKE unaccent(lower(?))", like),
			sq.And{visible, sq.Expr("unaccent(lower(u.email)) LIKE unaccent(lower(?))", like)},
		})
	}

	switch status := query.Get("status"); status {
	case "":
		where = append(where, sq.NotEq{"u.status": StatusOffboarded}) // RF-113
	case StatusActive, StatusInactive, StatusOffboarded:
		where = append(where, sq.Eq{"u.status": status})
	default:
		return DirectoryPage{}, apperr.ErrInvalidFilter.WithDetail("field", "status")
	}

	if v := query.Get("team_id"); v != "" {
		cond, err := s.teamFilter(ctx, v)
		if err != nil {
			return DirectoryPage{}, err
		}
		where = append(where, cond)
	}

	if v := query.Get("role"); v != "" {
		if _, ok := authz.ParseRole(v); !ok {
			return DirectoryPage{}, apperr.ErrInvalidFilter.WithDetail("field", "role")
		}
		where = append(where, visible, sq.Expr("EXISTS (SELECT 1 FROM user_roles r WHERE r.user_id = u.id AND r.role = ?)", v))
	}

	attrConds, err := s.attributeFilters(ctx, p, query)
	if err != nil {
		return DirectoryPage{}, err
	}
	if len(attrConds) > 0 {
		where = append(where, visible)
		where = append(where, attrConds...)
	}

	total, err := s.countDirectory(ctx, where)
	if err != nil {
		return DirectoryPage{}, err
	}
	sqlStr, args, err := psql.Select("u.id", "u.name", "u.email", "u.status").
		Column(sq.Alias(visible, "full_profile")).
		From("users u").Where(where).
		OrderBy("lower(u.name)", "u.id").
		Limit(uint64(page.Size)).Offset(uint64(page.Offset())).
		ToSql()
	if err != nil {
		return DirectoryPage{}, fmt.Errorf("construir directorio: %w", err)
	}
	rows, err := s.store.Pool.Query(ctx, sqlStr, args...)
	if err != nil {
		return DirectoryPage{}, fmt.Errorf("consultar directorio: %w", err)
	}
	defer rows.Close()

	out := DirectoryPage{Items: []User{}, Total: total, Page: page.Number, PageSize: page.Size}
	emails := map[uuid.UUID]string{}
	for rows.Next() {
		var u User
		var email string
		if err := rows.Scan(&u.ID, &u.Name, &email, &u.Status, &u.FullProfile); err != nil {
			return DirectoryPage{}, fmt.Errorf("leer directorio: %w", err)
		}
		emails[u.ID] = email
		out.Items = append(out.Items, u)
	}
	if err := rows.Err(); err != nil {
		return DirectoryPage{}, fmt.Errorf("leer directorio: %w", err)
	}
	return out, s.enrich(ctx, p, out.Items, emails)
}

// visibleFull is the SQL condition "the viewer sees this user's full
// profile": everyone for administrators; otherwise the viewer and the
// members or viewers of the teams it leads (with subteams).
func (s *Service) visibleFull(p *authz.Principal) sq.Sqlizer {
	if p.Has(authz.UsersReadFullAll) {
		return sq.Expr("TRUE")
	}
	scope := make([]string, 0, len(p.ScopeTeams))
	if p.Has(authz.UsersReadFullTeam) {
		for id := range p.ScopeTeams {
			scope = append(scope, id.String())
		}
	}
	return sq.Expr(`(u.id = ? OR (
		EXISTS (SELECT 1 FROM team_members tm WHERE tm.user_id = u.id AND tm.team_id = ANY(?::uuid[]))
		AND NOT EXISTS (SELECT 1 FROM user_roles r WHERE r.user_id = u.id AND r.role IN ('admin', 'lead'))))`,
		p.UserID.String(), scope)
}

func (s *Service) teamFilter(ctx context.Context, raw string) (sq.Sqlizer, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, apperr.ErrInvalidFilter.WithDetail("field", "team_id")
	}
	ids, err := s.store.TeamDescendantIDs(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("leer subequipos: %w", err)
	}
	if len(ids) == 0 {
		return nil, apperr.ErrInvalidFilter.WithDetail("field", "team_id")
	}
	strs := make([]string, len(ids))
	for i, t := range ids {
		strs[i] = t.String()
	}
	return sq.Expr("EXISTS (SELECT 1 FROM team_members tm WHERE tm.user_id = u.id AND tm.team_id = ANY(?::uuid[]))", strs), nil
}

func (s *Service) attributeFilters(ctx context.Context, p *authz.Principal, query url.Values) ([]sq.Sqlizer, error) {
	type bounds struct{ eq, from, to string }
	wanted := map[uuid.UUID]*bounds{}
	for key, vals := range query {
		rest, ok := strings.CutPrefix(key, "attr.")
		if !ok || len(vals) == 0 {
			continue
		}
		idStr, suffix, _ := strings.Cut(rest, ".")
		id, err := uuid.Parse(idStr)
		if err != nil {
			return nil, apperr.ErrInvalidFilter.WithDetail("field", key)
		}
		b := wanted[id]
		if b == nil {
			b = &bounds{}
			wanted[id] = b
		}
		switch suffix {
		case "":
			b.eq = vals[0]
		case "from":
			b.from = vals[0]
		case "to":
			b.to = vals[0]
		default:
			return nil, apperr.ErrInvalidFilter.WithDetail("field", key)
		}
	}
	if len(wanted) == 0 {
		return nil, nil
	}

	defs, err := attributes.Load(ctx, s.store.Queries)
	if err != nil {
		return nil, err
	}
	byID := map[uuid.UUID]attributes.Attribute{}
	for _, d := range defs {
		byID[d.ID] = d
	}

	var out []sq.Sqlizer
	for id, b := range wanted {
		a, ok := byID[id]
		invalid := apperr.ErrInvalidFilter.WithDetail("attribute", id.String())
		if !ok || (a.Retired() && !p.Has(authz.AttributesManage)) {
			return nil, invalid
		}
		base := "EXISTS (SELECT 1 FROM user_attribute_values v WHERE v.user_id = u.id AND v.attribute_id = ? AND "
		switch a.Type {
		case attributes.TypeText:
			if b.eq == "" {
				return nil, invalid
			}
			out = append(out, sq.Expr(base+"lower(v.value #>> '{}') = lower(?))", id.String(), b.eq))
		case attributes.TypeSelect:
			if b.eq == "" {
				return nil, invalid
			}
			out = append(out, sq.Expr(base+"v.value #>> '{}' = ?)", id.String(), b.eq))
		case attributes.TypeBoolean:
			if b.eq != "true" && b.eq != "false" {
				return nil, invalid
			}
			out = append(out, sq.Expr(base+"v.value = ?::jsonb)", id.String(), b.eq))
		case attributes.TypeNumber, attributes.TypeDate:
			cast, check := "numeric", func(v string) bool { _, err := strconv.ParseFloat(v, 64); return err == nil }
			if a.Type == attributes.TypeDate {
				cast, check = "date", func(v string) bool { _, err := time.Parse(time.DateOnly, v); return err == nil }
			}
			from, to := b.from, b.to
			if b.eq != "" {
				from, to = b.eq, b.eq
			}
			if from == "" && to == "" {
				return nil, invalid
			}
			conds := []string{}
			args := []any{id.String()}
			for _, bound := range []struct{ v, op string }{{from, ">="}, {to, "<="}} {
				if bound.v == "" {
					continue
				}
				if !check(bound.v) {
					return nil, invalid
				}
				conds = append(conds, fmt.Sprintf("(v.value #>> '{}')::%s %s ?::%s", cast, bound.op, cast))
				args = append(args, bound.v)
			}
			out = append(out, sq.Expr(base+strings.Join(conds, " AND ")+")", args...))
		default:
			return nil, invalid
		}
	}
	return out, nil
}

func (s *Service) countDirectory(ctx context.Context, where sq.And) (int64, error) {
	sqlStr, args, err := psql.Select("count(*)").From("users u").Where(where).ToSql()
	if err != nil {
		return 0, fmt.Errorf("construir recuento: %w", err)
	}
	var n int64
	if err := s.store.Pool.QueryRow(ctx, sqlStr, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("contar directorio: %w", err)
	}
	return n, nil
}

// enrich adds teams to every item and email, roles and active attributes to
// those the viewer fully sees.
func (s *Service) enrich(ctx context.Context, p *authz.Principal, items []User, emails map[uuid.UUID]string) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(items))
	index := map[uuid.UUID]int{}
	for i, u := range items {
		ids[i] = u.ID
		index[u.ID] = i
		items[i].Teams = []TeamRef{}
		if u.FullProfile {
			email := emails[u.ID]
			items[i].Email = &email
			items[i].Roles = []RoleRef{}
			items[i].Attributes = map[uuid.UUID]json.RawMessage{}
		}
	}
	teams, err := s.store.ListTeamsForUsers(ctx, ids)
	if err != nil {
		return fmt.Errorf("leer equipos: %w", err)
	}
	for _, t := range teams {
		i := index[t.UserID]
		items[i].Teams = append(items[i].Teams, TeamRef{ID: t.ID, Name: t.Name, Source: t.Source})
	}
	roles, err := s.store.ListRolesForUsers(ctx, ids)
	if err != nil {
		return fmt.Errorf("leer roles: %w", err)
	}
	targets := make([]authz.Target, len(items))
	for i, u := range items {
		targets[i].UserID = u.ID
	}
	for _, t := range teams {
		i := index[t.UserID]
		targets[i].TeamIDs = append(targets[i].TeamIDs, t.ID)
	}
	for _, r := range roles {
		i := index[r.UserID]
		targets[i].Roles = append(targets[i].Roles, authz.Role(r.Role))
		if items[i].FullProfile {
			items[i].Roles = append(items[i].Roles, RoleRef{Role: r.Role, Source: r.Source})
		}
	}
	// Rows say whether the viewer may edit them, for the table edited in place (RF-128).
	for i := range items {
		if items[i].FullProfile {
			items[i].Can = &Abilities{Edit: p.CanActOn(authz.UsersUpdateAll, authz.UsersUpdateTeam, targets[i])}
		}
	}
	defs, err := attributes.Load(ctx, s.store.Queries)
	if err != nil {
		return err
	}
	retired := map[uuid.UUID]bool{}
	for _, d := range defs {
		retired[d.ID] = d.Retired()
	}
	values, err := s.store.ListAttributeValuesForUsers(ctx, ids)
	if err != nil {
		return fmt.Errorf("leer atributos: %w", err)
	}
	for _, v := range values {
		if i := index[v.UserID]; items[i].FullProfile && !retired[v.AttributeID] {
			items[i].Attributes[v.AttributeID] = v.Value
		}
	}
	return nil
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
