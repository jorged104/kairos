package schedule

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/authz"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
)

// Palette is the closed set of colors of the levels (RF-40).
var Palette = []string{"blue", "green", "amber", "red", "orange", "violet", "teal", "slate"}

// Levels in order, from ahead to late.
var Levels = []Level{Ahead, OnTrack, AtRisk, Late}

// LevelStyle is the name and color of a level; a null name is the default
// one of the translations (RF-39).
type LevelStyle struct {
	Name  *string `json:"name"`
	Color string  `json:"color"`
}

// Settings of the traffic light, global (RF-32 to RF-41).
type Settings struct {
	Upper   float64              `json:"upper"`
	Risk    float64              `json:"risk"`
	Late    float64              `json:"late"`
	Levels  map[Level]LevelStyle `json:"levels"`
	Version int32                `json:"version"`
}

// DefaultSettings are the values of RF-32.
func DefaultSettings() Settings {
	return Settings{Upper: 5, Risk: -5, Late: -15, Version: 1, Levels: map[Level]LevelStyle{
		Ahead: {Color: "blue"}, OnTrack: {Color: "green"}, AtRisk: {Color: "amber"}, Late: {Color: "red"},
	}}
}

func toSettings(r sqlc.ScheduleSetting) Settings {
	return Settings{Upper: r.Upper, Risk: r.Risk, Late: r.Late, Version: r.Version, Levels: map[Level]LevelStyle{
		Ahead:   {Name: r.NameAhead, Color: r.ColorAhead},
		OnTrack: {Name: r.NameOnTrack, Color: r.ColorOnTrack},
		AtRisk:  {Name: r.NameAtRisk, Color: r.ColorAtRisk},
		Late:    {Name: r.NameLate, Color: r.ColorLate},
	}}
}

// Load reads the settings on every computation, so a change applies on the
// next query (RF-41).
func Load(ctx context.Context, q *sqlc.Queries) (Settings, error) {
	r, err := q.GetScheduleSettings(ctx)
	if err != nil {
		return Settings{}, fmt.Errorf("leer semáforo: %w", err)
	}
	return toSettings(r), nil
}

type Service struct{ store *db.Store }

func NewService(store *db.Store) *Service { return &Service{store: store} }

func (s *Service) Get(ctx context.Context) (Settings, error) { return Load(ctx, s.store.Queries) }

type settingsRequest struct {
	Upper   *float64             `json:"upper" validate:"required"`
	Risk    *float64             `json:"risk" validate:"required"`
	Late    *float64             `json:"late" validate:"required"`
	Levels  map[Level]LevelStyle `json:"levels" validate:"required"`
	Version int32                `json:"version" validate:"required"`
}

func (r *settingsRequest) Normalize() {
	for k, l := range r.Levels {
		if l.Name != nil {
			n := strings.TrimSpace(*l.Name)
			l.Name = &n
			if n == "" {
				l.Name = nil
			}
		}
		r.Levels[k] = l
	}
}

// check validates RF-37 to RF-40, saying which field fails.
func (r settingsRequest) check() error {
	for _, f := range []struct {
		name string
		v    float64
	}{{"upper", *r.Upper}, {"risk", *r.Risk}, {"late", *r.Late}} {
		if f.v < -100 || f.v > 100 { // RF-38
			return apperr.ErrValidation.WithDetail("field", f.name)
		}
	}
	if *r.Upper <= *r.Risk { // RF-37
		return apperr.ErrValidation.WithDetail("field", "upper")
	}
	if *r.Risk <= *r.Late {
		return apperr.ErrValidation.WithDetail("field", "risk")
	}
	for _, lv := range Levels {
		l, ok := r.Levels[lv]
		if !ok || !contains(Palette, l.Color) { // RF-40
			return apperr.ErrValidation.WithDetail("field", "levels."+string(lv)+".color")
		}
		if l.Name != nil && len([]rune(*l.Name)) > 30 { // RF-39
			return apperr.ErrValidation.WithDetail("field", "levels."+string(lv)+".name")
		}
	}
	if len(r.Levels) != len(Levels) {
		return apperr.ErrValidation.WithDetail("field", "levels")
	}
	return nil
}

// Update changes the settings, audited with before and after (RF-36, RF-50).
func (s *Service) Update(ctx context.Context, p *authz.Principal, r settingsRequest) (Settings, error) {
	if err := r.check(); err != nil {
		return Settings{}, err
	}
	var out Settings
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		cur, err := q.GetScheduleSettingsForUpdate(ctx)
		if err != nil {
			return fmt.Errorf("leer semáforo: %w", err)
		}
		if cur.Version != r.Version {
			return apperr.VersionConflict(cur.Version)
		}
		l := r.Levels
		row, err := q.UpdateScheduleSettings(ctx, sqlc.UpdateScheduleSettingsParams{
			Upper: *r.Upper, Risk: *r.Risk, Late: *r.Late,
			NameAhead: l[Ahead].Name, NameOnTrack: l[OnTrack].Name, NameAtRisk: l[AtRisk].Name, NameLate: l[Late].Name,
			ColorAhead: l[Ahead].Color, ColorOnTrack: l[OnTrack].Color, ColorAtRisk: l[AtRisk].Color, ColorLate: l[Late].Color,
		})
		if err != nil {
			return fmt.Errorf("guardar semáforo: %w", err)
		}
		out = toSettings(row)
		return audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "schedule_settings.updated",
			TargetType: "schedule_settings", TargetID: "global", Before: toSettings(cur), After: out})
	})
	return out, err
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
