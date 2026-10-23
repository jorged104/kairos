package attributes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"openscum/internal/db/sqlc"
)

// Values maps attribute ids to JSON values; a JSON null removes the value.
type Values map[uuid.UUID]json.RawMessage

// Normalize checks raw against the attribute type and returns the value to
// store, or nil when it clears the attribute (RF-97). previous lets a user
// keep a select option that has been retired since (RF-107).
func Normalize(a Attribute, raw, previous json.RawMessage) (json.RawMessage, error) {
	invalid := ErrInvalidValue.WithDetail("attribute", a.ID.String())
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	switch a.Type {
	case TypeText:
		var s string
		if json.Unmarshal(raw, &s) != nil || utf8.RuneCountInString(s) > maxTextLength {
			return nil, invalid
		}
		if s = strings.TrimSpace(s); s == "" {
			return nil, nil
		}
		return json.Marshal(s)
	case TypeNumber:
		var f float64
		if json.Unmarshal(raw, &f) != nil {
			return nil, invalid
		}
		return json.Marshal(f)
	case TypeDate:
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return nil, invalid
		}
		if _, err := time.Parse(time.DateOnly, s); err != nil {
			return nil, invalid
		}
		return json.Marshal(s)
	case TypeBoolean:
		var b bool
		if json.Unmarshal(raw, &b) != nil {
			return nil, invalid
		}
		return json.Marshal(b)
	case TypeSelect:
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return nil, invalid
		}
		if prev := previousString(previous); prev == s {
			return json.Marshal(s)
		}
		for _, o := range a.Options {
			if o.Value == s && !o.Retired {
				return json.Marshal(s)
			}
		}
		return nil, invalid
	}
	return nil, invalid
}

func previousString(previous json.RawMessage) string {
	var s string
	if len(previous) == 0 || json.Unmarshal(previous, &s) != nil {
		return ""
	}
	return s
}

// Apply validates and saves a user's attribute values in the caller's
// transaction. With requireAll, every active required attribute must end up
// with a value (RF-98, RF-99); JIT creation skips it (RF-108).
func Apply(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, values Values, requireAll bool) error {
	defs, err := Load(ctx, q)
	if err != nil {
		return err
	}
	current, err := currentValues(ctx, q, userID)
	if err != nil {
		return err
	}
	byID := make(map[uuid.UUID]Attribute, len(defs))
	for _, a := range defs {
		byID[a.ID] = a
	}

	for id, raw := range values {
		a, ok := byID[id]
		if !ok {
			return ErrInvalidValue.WithDetail("attribute", id.String())
		}
		if a.Retired() {
			// Retired values are read-only history (RF-104).
			if bytes.Equal(bytes.TrimSpace(raw), current[id]) {
				continue
			}
			return errRetired.WithDetail("attribute", id.String())
		}
		v, err := Normalize(a, raw, current[id])
		if err != nil {
			return err
		}
		if v == nil {
			if err := q.DeleteUserAttributeValue(ctx, sqlc.DeleteUserAttributeValueParams{UserID: userID, AttributeID: id}); err != nil {
				return fmt.Errorf("borrar valor: %w", err)
			}
			delete(current, id)
			continue
		}
		if err := q.UpsertUserAttributeValue(ctx, sqlc.UpsertUserAttributeValueParams{UserID: userID, AttributeID: id, Value: v}); err != nil {
			return fmt.Errorf("guardar valor: %w", err)
		}
		current[id] = v
	}

	if requireAll {
		for _, a := range defs {
			if a.Required && !a.Retired() && current[a.ID] == nil {
				return ErrRequired.WithDetail("attribute", a.ID.String())
			}
		}
	}
	return nil
}

func currentValues(ctx context.Context, q *sqlc.Queries, userID uuid.UUID) (map[uuid.UUID]json.RawMessage, error) {
	rows, err := q.ListUserAttributeValues(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("leer valores: %w", err)
	}
	out := make(map[uuid.UUID]json.RawMessage, len(rows))
	for _, r := range rows {
		out[r.AttributeID] = r.Value
	}
	return out, nil
}

// Load returns every attribute definition, retired included.
func Load(ctx context.Context, q *sqlc.Queries) ([]Attribute, error) {
	rows, err := q.ListAttributes(ctx)
	if err != nil {
		return nil, fmt.Errorf("listar atributos: %w", err)
	}
	out := make([]Attribute, 0, len(rows))
	for _, r := range rows {
		a, err := toAttribute(r)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

func toAttribute(r sqlc.Attribute) (Attribute, error) {
	a := Attribute{ID: r.ID, Name: r.Name, Type: r.Type, Required: r.Required, RetiredAt: r.RetiredAt, Options: []Option{}}
	if len(r.Options) > 0 {
		if err := json.Unmarshal(r.Options, &a.Options); err != nil {
			return a, fmt.Errorf("leer opciones del atributo %s: %w", r.ID, err)
		}
	}
	return a, nil
}
