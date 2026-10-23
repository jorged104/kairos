// Package workcore holds small pieces shared by the spec 004 modules: dates in
// JSON, people that can own or take work, and the "can" flags of a project.
package workcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/authz"
	"openscum/internal/db/sqlc"
)

// Date is a calendar date written as YYYY-MM-DD in JSON.
type Date struct{ time.Time }

func (d Date) MarshalJSON() ([]byte, error) { return json.Marshal(d.Format(time.DateOnly)) }

func (d *Date) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return err
	}
	d.Time = t
	return nil
}

// ToTime converts an optional date for the queries.
func ToTime(d *Date) *time.Time {
	if d == nil {
		return nil
	}
	t := d.Time
	return &t
}

// FromTime converts an optional date from the queries.
func FromTime(t *time.Time) *Date {
	if t == nil {
		return nil
	}
	return &Date{*t}
}

// CheckRange rejects an end before the start when both are set.
func CheckRange(start, end *Date, err *apperr.Error) error {
	if start != nil && end != nil && end.Before(start.Time) {
		return err
	}
	return nil
}

// CheckWorker accepts users that can own or take new work: active and with a
// role other than viewer (RF-25, RF-68). fail is returned otherwise.
func CheckWorker(ctx context.Context, q *sqlc.Queries, id uuid.UUID, fail *apperr.Error) error {
	u, err := q.GetUser(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail
	}
	if err != nil {
		return fmt.Errorf("leer usuario: %w", err)
	}
	if u.Status != "active" {
		return fail
	}
	roles, err := q.ListUserRoles(ctx, id)
	if err != nil {
		return fmt.Errorf("leer roles: %w", err)
	}
	for _, r := range roles {
		if authz.Role(r.Role) != authz.RoleViewer {
			return nil
		}
	}
	return fail
}

// Ref is a short reference to a related record.
type Ref struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// EscapeLike escapes the LIKE wildcards of user input.
func EscapeLike(s string) string {
	return strings.NewReplacer(`\`, `\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
