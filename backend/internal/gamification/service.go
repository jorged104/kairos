package gamification

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/auth"
	"openscum/internal/authz"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
	"openscum/pkg/httpx"
)

const historyPageSize = 50 // RF-39

var (
	errAdjustEmpty  = apperr.New(http.StatusBadRequest, "points_adjust_empty", "El ajuste no cambia ni XP ni monedas")
	errAdjustSelf   = apperr.New(http.StatusForbidden, "points_adjust_self", "No puedes ajustar tus propios puntos")
	errNegative     = apperr.New(http.StatusConflict, "points_negative", "El ajuste dejaría el saldo por debajo de 0")
	errLevelExists  = apperr.New(http.StatusConflict, "level_conflict", "Ya hay un nivel con ese XP mínimo")
	errLevelLocked  = apperr.New(http.StatusConflict, "level_locked", "El primer nivel empieza siempre en 0 XP")
	errUnknownSize  = apperr.New(http.StatusBadRequest, "validation_failed", "Talla desconocida").WithDetail("field", "size_id")
	errInvalidRange = apperr.ErrInvalidFilter.WithDetail("field", "period")
)

type Level struct {
	ID     uuid.UUID `json:"id"`
	Number int       `json:"number"`
	Name   string    `json:"name"`
	MinXP  int32     `json:"min_xp"`
}

// Summary is a person's balance and where it puts them (RF-3, RF-4, RF-29,
// RF-37, RF-38). LastSeq lets the interface notice new movements (RF-41).
type Summary struct {
	XP      int64  `json:"xp"`
	Coins   int64  `json:"coins"`
	Level   Level  `json:"level"`
	Next    *Level `json:"next"`
	LastSeq int64  `json:"last_seq"`
}

type WorkItemRef struct {
	ID    uuid.UUID `json:"id"`
	Key   string    `json:"key"`
	Title string    `json:"title"`
}

type Movement struct {
	ID        uuid.UUID    `json:"id"`
	Seq       int64        `json:"seq"`
	XP        int32        `json:"xp"`
	Coins     int32        `json:"coins"`
	Reason    string       `json:"reason"`
	Note      *string      `json:"note"`
	CreatedAt time.Time    `json:"created_at"`
	WorkItem  *WorkItemRef `json:"work_item"`
}

type History struct {
	Items    []Movement `json:"items"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
}

type Value struct {
	XP    int32 `json:"xp" validate:"min=0,max=1000"`
	Coins int32 `json:"coins" validate:"min=0,max=1000"`
}

type SizeValue struct {
	SizeID  uuid.UUID `json:"size_id"`
	Name    string    `json:"name"`
	Retired bool      `json:"retired"`
	Value
}

// Values is the table of points (RF-24 to RF-27).
type Values struct {
	NoSize Value       `json:"no_size"`
	Sizes  []SizeValue `json:"sizes"`
}

type SetValuesRequest struct {
	NoSize Value `json:"no_size"`
	Sizes  []struct {
		SizeID uuid.UUID `json:"size_id" validate:"required"`
		Value
	} `json:"sizes" validate:"dive"`
}

type LevelRequest struct {
	Name  string `json:"name" validate:"required,max=40"`
	MinXP int32  `json:"min_xp" validate:"min=0"`
}

func (r *LevelRequest) Normalize() { r.Name = strings.TrimSpace(r.Name) }

type AdjustRequest struct {
	XP    int32  `json:"xp" validate:"min=-100000,max=100000"`
	Coins int32  `json:"coins" validate:"min=-100000,max=100000"`
	Note  string `json:"note" validate:"min=3,max=500"`
}

func (r *AdjustRequest) Normalize() { r.Note = strings.TrimSpace(r.Note) }

type Ref struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type RankingRow struct {
	Position int   `json:"position"`
	User     Ref   `json:"user"`
	Level    Level `json:"level"`
	XP       int64 `json:"xp"`
}

type Ranking struct {
	Team   Ref          `json:"team"`
	Period string       `json:"period"`
	From   *string      `json:"from"`
	To     *string      `json:"to"`
	Items  []RankingRow `json:"items"`
}

type Service struct {
	store *db.Store
}

func NewService(store *db.Store) *Service { return &Service{store: store} }

// levels returns the table numbered from 1 by minimum XP (RF-33).
func levels(ctx context.Context, q *sqlc.Queries) ([]Level, error) {
	rows, err := q.ListLevels(ctx)
	if err != nil {
		return nil, fmt.Errorf("leer niveles: %w", err)
	}
	out := make([]Level, len(rows))
	for i, r := range rows {
		out[i] = Level{ID: r.ID, Number: i + 1, Name: r.Name, MinXP: r.MinXp}
	}
	return out, nil
}

// levelOf is the level of highest minimum not above xp (RF-29); a negative
// balance stays in the first one.
func levelOf(ls []Level, xp int64) (Level, *Level) {
	i := 0
	for j, l := range ls {
		if int64(l.MinXP) <= xp {
			i = j
		}
	}
	var next *Level
	if i+1 < len(ls) {
		n := ls[i+1]
		next = &n
	}
	if len(ls) == 0 {
		return Level{Number: 1}, nil
	}
	return ls[i], next
}

func (s *Service) summary(ctx context.Context, q *sqlc.Queries, user uuid.UUID) (Summary, error) {
	b, err := q.UserPointsBalance(ctx, user)
	if err != nil {
		return Summary{}, fmt.Errorf("leer saldo: %w", err)
	}
	ls, err := levels(ctx, q)
	if err != nil {
		return Summary{}, err
	}
	level, next := levelOf(ls, b.Xp)
	return Summary{XP: b.Xp, Coins: b.Coins, Level: level, Next: next, LastSeq: b.LastSeq}, nil
}

// Summary of the caller, or of anybody for a manager (RF-40).
func (s *Service) Summary(ctx context.Context, p *authz.Principal, user uuid.UUID) (Summary, error) {
	if err := s.canSee(ctx, p, user); err != nil {
		return Summary{}, err
	}
	return s.summary(ctx, s.store.Queries, user)
}

func (s *Service) canSee(ctx context.Context, p *authz.Principal, user uuid.UUID) error {
	if user == p.UserID {
		return nil
	}
	if !p.Has(authz.GamificationManage) {
		return apperr.ErrForbidden
	}
	if _, err := s.store.GetUser(ctx, user); errors.Is(err, pgx.ErrNoRows) {
		return apperr.ErrNotFound
	} else if err != nil {
		return fmt.Errorf("leer usuario: %w", err)
	}
	return nil
}

// History pages the movements newest first (RF-39); with afterSeq it returns
// the first page of those after it instead, for the notice (RF-41).
func (s *Service) History(ctx context.Context, p *authz.Principal, user uuid.UUID, page httpx.Page, afterSeq int64) (History, error) {
	if err := s.canSee(ctx, p, user); err != nil {
		return History{}, err
	}
	rows, err := s.store.ListUserMovements(ctx, sqlc.ListUserMovementsParams{
		UserID: user, AfterSeq: afterSeq, Lim: int32(page.Size), Off: int32(page.Offset()),
	})
	if err != nil {
		return History{}, fmt.Errorf("leer movimientos: %w", err)
	}
	total, err := s.store.CountUserMovements(ctx, user)
	if err != nil {
		return History{}, fmt.Errorf("contar movimientos: %w", err)
	}
	out := History{Items: make([]Movement, len(rows)), Total: total, Page: page.Number, PageSize: page.Size}
	for i, r := range rows {
		m := Movement{ID: r.ID, Seq: r.Seq, XP: r.Xp, Coins: r.Coins, Reason: r.Reason, Note: r.Note, CreatedAt: r.CreatedAt}
		if r.WorkItemID != nil && r.WorkItemTitle != nil {
			m.WorkItem = &WorkItemRef{ID: *r.WorkItemID, Key: r.WorkItemKey, Title: *r.WorkItemTitle}
		}
		out.Items[i] = m
	}
	return out, nil
}

// Adjust adds a manual movement with its reason (RF-18 to RF-23).
func (s *Service) Adjust(ctx context.Context, p *authz.Principal, user uuid.UUID, req AdjustRequest) (Summary, error) {
	if !p.Has(authz.GamificationManage) {
		return Summary{}, apperr.ErrForbidden
	}
	if req.XP == 0 && req.Coins == 0 {
		return Summary{}, errAdjustEmpty
	}
	if user == p.UserID {
		return Summary{}, errAdjustSelf
	}
	var out Summary
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		if _, err := q.GetUser(ctx, user); errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		} else if err != nil {
			return fmt.Errorf("leer usuario: %w", err)
		}
		if err := q.LockUserPoints(ctx, user); err != nil {
			return fmt.Errorf("bloquear saldo: %w", err)
		}
		b, err := q.UserPointsBalance(ctx, user)
		if err != nil {
			return fmt.Errorf("leer saldo: %w", err)
		}
		if b.Xp+int64(req.XP) < 0 || b.Coins+int64(req.Coins) < 0 {
			return errNegative
		}
		actor := p.UserID
		if _, err := q.InsertPointMovement(ctx, sqlc.InsertPointMovementParams{
			UserID: user, Xp: req.XP, Coins: req.Coins, Reason: reasonAdjust, Note: &req.Note, ActorID: &actor, CreatedAt: time.Now(),
		}); err != nil {
			return fmt.Errorf("registrar ajuste: %w", err)
		}
		if err := audit.Record(ctx, q, audit.Entry{
			Actor: audit.ByUser(p), Action: "points.adjusted", TargetType: "user", TargetID: user.String(),
			After: map[string]any{"xp": req.XP, "coins": req.Coins, "note": req.Note},
		}); err != nil {
			return err
		}
		out, err = s.summary(ctx, q, user)
		return err
	})
	return out, err
}

func readValues(ctx context.Context, q *sqlc.Queries) (Values, error) {
	none, err := q.GetPointValue(ctx, nil)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Values{}, fmt.Errorf("leer valor sin talla: %w", err)
	}
	rows, err := q.ListPointValues(ctx)
	if err != nil {
		return Values{}, fmt.Errorf("leer tabla de puntos: %w", err)
	}
	out := Values{NoSize: Value{XP: none.Xp, Coins: none.Coins}, Sizes: make([]SizeValue, len(rows))}
	for i, r := range rows {
		out.Sizes[i] = SizeValue{SizeID: r.ID, Name: r.Name, Retired: r.RetiredAt != nil, Value: Value{XP: r.Xp, Coins: r.Coins}}
	}
	return out, nil
}

func (s *Service) Values(ctx context.Context) (Values, error) {
	return readValues(ctx, s.store.Queries)
}

// SetValues replaces the given values of the table, audited (RF-24, RF-28).
func (s *Service) SetValues(ctx context.Context, p *authz.Principal, req SetValuesRequest) (Values, error) {
	if !p.Has(authz.GamificationManage) {
		return Values{}, apperr.ErrForbidden
	}
	var out Values
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		before, err := readValues(ctx, q)
		if err != nil {
			return err
		}
		if err := q.SetNoSizePointValue(ctx, sqlc.SetNoSizePointValueParams{Xp: req.NoSize.XP, Coins: req.NoSize.Coins}); err != nil {
			return fmt.Errorf("guardar valor sin talla: %w", err)
		}
		for _, v := range req.Sizes {
			if _, err := q.GetSize(ctx, v.SizeID); errors.Is(err, pgx.ErrNoRows) {
				return errUnknownSize
			} else if err != nil {
				return fmt.Errorf("leer talla: %w", err)
			}
			id := v.SizeID
			if err := q.SetSizePointValue(ctx, sqlc.SetSizePointValueParams{SizeID: &id, Xp: v.XP, Coins: v.Coins}); err != nil {
				return fmt.Errorf("guardar valor: %w", err)
			}
		}
		out, err = readValues(ctx, q)
		if err != nil {
			return err
		}
		return audit.Record(ctx, q, audit.Entry{
			Actor: audit.ByUser(p), Action: "point_values.updated", TargetType: "point_values", TargetID: "point_values",
			Before: before, After: out,
		})
	})
	return out, err
}

func (s *Service) Levels(ctx context.Context) ([]Level, error) { return levels(ctx, s.store.Queries) }

// CreateLevel, UpdateLevel and DeleteLevel edit the table of levels (RF-30 to
// RF-36); levels are derived, so nobody's balance moves.
func (s *Service) CreateLevel(ctx context.Context, p *authz.Principal, req LevelRequest) (Level, error) {
	return s.editLevel(ctx, p, func(q *sqlc.Queries) (uuid.UUID, string, any, any, error) {
		row, err := q.CreateLevel(ctx, sqlc.CreateLevelParams{Name: req.Name, MinXp: req.MinXP})
		return row.ID, "level.created", nil, req, err
	})
}

func (s *Service) UpdateLevel(ctx context.Context, p *authz.Principal, id uuid.UUID, req LevelRequest) (Level, error) {
	return s.editLevel(ctx, p, func(q *sqlc.Queries) (uuid.UUID, string, any, any, error) {
		old, err := q.GetLevel(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return id, "", nil, nil, apperr.ErrNotFound
		}
		if err != nil {
			return id, "", nil, nil, fmt.Errorf("leer nivel: %w", err)
		}
		if old.MinXp == 0 && req.MinXP != 0 {
			return id, "", nil, nil, errLevelLocked // RF-31
		}
		_, err = q.UpdateLevel(ctx, sqlc.UpdateLevelParams{ID: id, Name: req.Name, MinXp: req.MinXP})
		return id, "level.updated", LevelRequest{Name: old.Name, MinXP: old.MinXp}, req, err
	})
}

func (s *Service) DeleteLevel(ctx context.Context, p *authz.Principal, id uuid.UUID) error {
	_, err := s.editLevel(ctx, p, func(q *sqlc.Queries) (uuid.UUID, string, any, any, error) {
		old, err := q.GetLevel(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return id, "", nil, nil, apperr.ErrNotFound
		}
		if err != nil {
			return id, "", nil, nil, fmt.Errorf("leer nivel: %w", err)
		}
		if old.MinXp == 0 {
			return id, "", nil, nil, errLevelLocked // RF-31
		}
		return id, "level.deleted", LevelRequest{Name: old.Name, MinXP: old.MinXp}, nil, q.DeleteLevel(ctx, id)
	})
	return err
}

func (s *Service) editLevel(ctx context.Context, p *authz.Principal, fn func(q *sqlc.Queries) (uuid.UUID, string, any, any, error)) (Level, error) {
	if !p.Has(authz.GamificationManage) {
		return Level{}, apperr.ErrForbidden
	}
	var out Level
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		if err := q.LockLevels(ctx); err != nil {
			return fmt.Errorf("bloquear niveles: %w", err)
		}
		id, action, before, after, err := fn(q)
		if auth.IsUniqueViolation(err) {
			return errLevelExists // RF-32
		}
		if err != nil {
			return err
		}
		ls, err := levels(ctx, q)
		if err != nil {
			return err
		}
		for _, l := range ls {
			if l.ID == id {
				out = l
			}
		}
		return audit.Record(ctx, q, audit.Entry{
			Actor: audit.ByUser(p), Action: action, TargetType: "level", TargetID: id.String(), Before: before, After: after,
		})
	})
	return out, err
}

// period returns the bounds of the ranking: the week from Monday or the
// calendar month that hold today, or none for «siempre» (RF-47).
func period(name string, today time.Time) (*time.Time, *time.Time, error) {
	d := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	switch name {
	case "week":
		from := d.AddDate(0, 0, -((int(d.Weekday()) + 6) % 7))
		to := from.AddDate(0, 0, 7)
		return &from, &to, nil
	case "", "month":
		from := time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.UTC)
		to := from.AddDate(0, 1, 0)
		return &from, &to, nil
	case "all":
		return nil, nil, nil
	}
	return nil, nil, errInvalidRange
}

// Ranking ranks the active members of a team and its subteams by their XP in
// the period (RF-43 to RF-52).
func (s *Service) Ranking(ctx context.Context, p *authz.Principal, teamID uuid.UUID, name string, today time.Time) (Ranking, error) {
	from, to, err := period(name, today)
	if err != nil {
		return Ranking{}, err
	}
	if name == "" {
		name = "month" // RF-48
	}
	team, err := s.store.GetTeam(ctx, teamID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Ranking{}, apperr.ErrNotFound
	}
	if err != nil {
		return Ranking{}, fmt.Errorf("leer equipo: %w", err)
	}
	if !p.Has(authz.GamificationManage) {
		_, err := s.store.GetTeamMember(ctx, sqlc.GetTeamMemberParams{TeamID: teamID, UserID: p.UserID})
		if errors.Is(err, pgx.ErrNoRows) {
			return Ranking{}, apperr.ErrForbidden // RF-52
		}
		if err != nil {
			return Ranking{}, fmt.Errorf("leer pertenencia: %w", err)
		}
	}
	ids, err := s.store.TeamDescendantIDs(ctx, teamID)
	if err != nil {
		return Ranking{}, fmt.Errorf("leer subequipos: %w", err)
	}
	rows, err := s.store.TeamRanking(ctx, sqlc.TeamRankingParams{FromAt: from, ToAt: to, TeamIds: ids})
	if err != nil {
		return Ranking{}, fmt.Errorf("leer ranking: %w", err)
	}
	ls, err := levels(ctx, s.store.Queries)
	if err != nil {
		return Ranking{}, err
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].PeriodXp != rows[j].PeriodXp {
			return rows[i].PeriodXp > rows[j].PeriodXp
		}
		return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name)
	})
	out := Ranking{Team: Ref{ID: team.ID, Name: team.Name}, Period: name, Items: make([]RankingRow, len(rows))}
	if from != nil {
		f, t := from.Format(time.DateOnly), to.AddDate(0, 0, -1).Format(time.DateOnly)
		out.From, out.To = &f, &t
	}
	for i, r := range rows {
		level, _ := levelOf(ls, r.TotalXp)
		out.Items[i] = RankingRow{Position: i + 1, User: Ref{ID: r.ID, Name: r.Name}, Level: level, XP: r.PeriodXp}
	}
	return out, nil
}
