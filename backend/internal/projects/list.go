package projects

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	"openscum/internal/apperr"
	"openscum/internal/workcore"
	"openscum/pkg/httpx"
)

const (
	DefaultPageSize = 25
	MaxPageSize     = 100
)

type Page struct {
	Items    []Summary `json:"items"`
	Total    int64     `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
}

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

// List returns projects sorted by name (RF-128). Query parameters: q,
// team_id (subteams included), owner_id, lifecycle, health, methodology_id,
// page, page_size. Archived projects only show with lifecycle=archivado (RF-30);
// templates never show here (RF-35).
func (s *Service) List(ctx context.Context, query url.Values) (Page, error) {
	page, ok := httpx.ParsePage(query, DefaultPageSize, MaxPageSize)
	if !ok {
		return Page{}, apperr.ErrInvalidFilter.WithDetail("field", "page")
	}
	where := sq.And{sq.Eq{"p.is_template": false}}
	if text := strings.TrimSpace(query.Get("q")); text != "" {
		like := "%" + workcore.EscapeLike(text) + "%"
		where = append(where, sq.Or{
			sq.Expr("unaccent(lower(p.name)) LIKE unaccent(lower(?))", like),
			sq.Expr("p.identifier LIKE upper(?)", like),
		})
	}
	switch v := query.Get("lifecycle"); v {
	case "":
		where = append(where, sq.NotEq{"p.lifecycle": LifecycleArchived})
	case LifecycleActive, LifecycleClosed, LifecycleArchived:
		where = append(where, sq.Eq{"p.lifecycle": v})
	default:
		return Page{}, apperr.ErrInvalidFilter.WithDetail("field", "lifecycle")
	}
	switch v := query.Get("health"); v {
	case "":
	case "en_curso", "en_riesgo", "fuera_de_curso":
		where = append(where, sq.Eq{"p.health": v})
	default:
		return Page{}, apperr.ErrInvalidFilter.WithDetail("field", "health")
	}
	for param, col := range map[string]string{"owner_id": "p.owner_id", "methodology_id": "p.methodology_id"} {
		if v := query.Get(param); v != "" {
			id, err := uuid.Parse(v)
			if err != nil {
				return Page{}, apperr.ErrInvalidFilter.WithDetail("field", param)
			}
			where = append(where, sq.Eq{col: id})
		}
	}
	if v := query.Get("team_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return Page{}, apperr.ErrInvalidFilter.WithDetail("field", "team_id")
		}
		ids, err := s.store.TeamDescendantIDs(ctx, id)
		if err != nil {
			return Page{}, fmt.Errorf("leer subequipos: %w", err)
		}
		if len(ids) == 0 {
			return Page{}, apperr.ErrInvalidFilter.WithDetail("field", "team_id")
		}
		strs := make([]string, len(ids))
		for i, t := range ids {
			strs[i] = t.String()
		}
		where = append(where, sq.Expr("p.team_id = ANY(?::uuid[])", strs))
	}

	countSQL, countArgs, err := psql.Select("count(*)").From("projects p").Where(where).ToSql()
	if err != nil {
		return Page{}, fmt.Errorf("construir conteo: %w", err)
	}
	var total int64
	if err := s.store.Pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return Page{}, fmt.Errorf("contar proyectos: %w", err)
	}

	listSQL, args, err := psql.Select(
		"p.id", "p.identifier", "p.name", "p.team_id", "t.name", "p.owner_id", "u.name",
		"p.methodology_id", "m.name", "p.parent_id", "p.lifecycle", "p.health",
		"p.start_date", "p.end_date", "p.depth", "p.updated_at",
	).From("projects p").
		Join("teams t ON t.id = p.team_id").
		Join("users u ON u.id = p.owner_id").
		LeftJoin("methodologies m ON m.id = p.methodology_id").
		Where(where).OrderBy("p.name", "p.identifier").
		Limit(uint64(page.Size)).Offset(uint64(page.Offset())).ToSql()
	if err != nil {
		return Page{}, fmt.Errorf("construir listado: %w", err)
	}
	rows, err := s.store.Pool.Query(ctx, listSQL, args...)
	if err != nil {
		return Page{}, fmt.Errorf("listar proyectos: %w", err)
	}
	defer rows.Close()
	out := Page{Items: []Summary{}, Total: total, Page: page.Number, PageSize: page.Size}
	for rows.Next() {
		var sm Summary
		var methodologyID *uuid.UUID
		var methodologyName *string
		var start, end *time.Time
		if err := rows.Scan(&sm.ID, &sm.Identifier, &sm.Name, &sm.Team.ID, &sm.Team.Name, &sm.Owner.ID, &sm.Owner.Name,
			&methodologyID, &methodologyName, &sm.ParentID, &sm.Lifecycle, &sm.Health, &start, &end, &sm.Depth, &sm.UpdatedAt); err != nil {
			return Page{}, fmt.Errorf("leer proyecto: %w", err)
		}
		if methodologyID != nil && methodologyName != nil {
			sm.Methodology = &workcore.Ref{ID: *methodologyID, Name: *methodologyName}
		}
		sm.StartDate, sm.EndDate = workcore.FromTime(start), workcore.FromTime(end)
		out.Items = append(out.Items, sm)
	}
	return out, rows.Err()
}
