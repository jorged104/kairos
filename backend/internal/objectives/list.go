package objectives

import (
	"context"
	"fmt"
	"net/url"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	"openscum/internal/apperr"
	"openscum/internal/workcore"
	"openscum/pkg/httpx"
)

type Page struct {
	Items    []Summary `json:"items"`
	Total    int64     `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
}

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

// List returns objectives by period and title (RF-131). Query parameters:
// status, scope, owner_id, from and to (periods overlapping that range),
// page, page_size.
func (s *Service) List(ctx context.Context, query url.Values) (Page, error) {
	page, ok := httpx.ParsePage(query, 25, 100)
	if !ok {
		return Page{}, apperr.ErrInvalidFilter.WithDetail("field", "page")
	}
	where := sq.And{}
	switch v := query.Get("status"); v {
	case "":
	case StatusDraft, StatusActive, StatusClosed, StatusCancelled:
		where = append(where, sq.Eq{"o.status": v})
	default:
		return Page{}, apperr.ErrInvalidFilter.WithDetail("field", "status")
	}
	switch v := query.Get("scope"); v {
	case "":
	case ScopeOrganization, ScopeTeam, ScopeProject:
		where = append(where, sq.Eq{"o.scope": v})
	default:
		return Page{}, apperr.ErrInvalidFilter.WithDetail("field", "scope")
	}
	if v := query.Get("owner_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return Page{}, apperr.ErrInvalidFilter.WithDetail("field", "owner_id")
		}
		where = append(where, sq.Eq{"o.owner_id": id})
	}
	for param, cond := range map[string]string{"from": "o.period_end >= ?", "to": "o.period_start <= ?"} {
		if v := query.Get(param); v != "" {
			d, err := time.Parse(time.DateOnly, v)
			if err != nil {
				return Page{}, apperr.ErrInvalidFilter.WithDetail("field", param)
			}
			where = append(where, sq.Expr(cond, d))
		}
	}

	countSQL, countArgs, err := psql.Select("count(*)").From("objectives o").Where(where).ToSql()
	if err != nil {
		return Page{}, fmt.Errorf("construir conteo: %w", err)
	}
	var total int64
	if err := s.store.Pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return Page{}, fmt.Errorf("contar objetivos: %w", err)
	}
	listSQL, args, err := psql.Select("o.id", "o.title", "o.owner_id", "u.name", "o.scope", "o.period_start", "o.period_end", "o.status").
		From("objectives o").Join("users u ON u.id = o.owner_id").Where(where).
		OrderBy("o.period_start DESC", "o.title").Limit(uint64(page.Size)).Offset(uint64(page.Offset())).ToSql()
	if err != nil {
		return Page{}, fmt.Errorf("construir listado: %w", err)
	}
	rows, err := s.store.Pool.Query(ctx, listSQL, args...)
	if err != nil {
		return Page{}, fmt.Errorf("listar objetivos: %w", err)
	}
	defer rows.Close()
	out := Page{Items: []Summary{}, Total: total, Page: page.Number, PageSize: page.Size}
	ids := []uuid.UUID{}
	for rows.Next() {
		var sm Summary
		var start, end time.Time
		if err := rows.Scan(&sm.ID, &sm.Title, &sm.Owner.ID, &sm.Owner.Name, &sm.Scope, &start, &end, &sm.Status); err != nil {
			return Page{}, fmt.Errorf("leer objetivo: %w", err)
		}
		sm.PeriodStart, sm.PeriodEnd = workcore.Date{Time: start}, workcore.Date{Time: end}
		out.Items = append(out.Items, sm)
		ids = append(ids, sm.ID)
	}
	if err := rows.Err(); err != nil {
		return Page{}, err
	}
	// Progress of the whole page in one query.
	krs, err := s.store.KeyResultsWithCounts(ctx, ids)
	if err != nil {
		return Page{}, fmt.Errorf("leer resultados clave: %w", err)
	}
	byObjective := map[uuid.UUID][]KeyResult{}
	for _, r := range krs {
		byObjective[r.ObjectiveID] = append(byObjective[r.ObjectiveID], keyResultView(r))
	}
	for i := range out.Items {
		out.Items[i].Progress = average(byObjective[out.Items[i].ID])
	}
	return out, nil
}
