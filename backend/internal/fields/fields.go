// Package fields holds the value types shared by the user attributes (spec
// 003) and the custom fields of projects and work items (spec 004).
package fields

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"openscum/internal/apperr"
)

const (
	TypeText    = "text"
	TypeNumber  = "number"
	TypeDate    = "date"
	TypeBoolean = "boolean"
	TypeSelect  = "select"
)

const maxTextLength = 1000

// Option is a choice of a select field. Retired options are kept for the
// records that already have them but cannot be chosen again.
type Option struct {
	Value   string `json:"value" validate:"required,max=200"`
	Retired bool   `json:"retired"`
}

var ErrOptionNeeded = apperr.New(http.StatusBadRequest, "option_required", "Un campo de selección necesita al menos una opción")

// Normalize checks raw against the field type and returns the value to store,
// or nil when it clears the field. ok is false when the value is invalid.
// previous lets a record keep a select option that has been retired since.
func Normalize(typ string, options []Option, raw, previous json.RawMessage) (value json.RawMessage, ok bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil, true
	}
	switch typ {
	case TypeText:
		var s string
		if json.Unmarshal(raw, &s) != nil || utf8.RuneCountInString(s) > maxTextLength {
			return nil, false
		}
		if s = strings.TrimSpace(s); s == "" {
			return nil, true
		}
		return marshal(s)
	case TypeNumber:
		var f float64
		if json.Unmarshal(raw, &f) != nil {
			return nil, false
		}
		return marshal(f)
	case TypeDate:
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return nil, false
		}
		if _, err := time.Parse(time.DateOnly, s); err != nil {
			return nil, false
		}
		return marshal(s)
	case TypeBoolean:
		var b bool
		if json.Unmarshal(raw, &b) != nil {
			return nil, false
		}
		return marshal(b)
	case TypeSelect:
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return nil, false
		}
		if prev := previousString(previous); prev == s {
			return marshal(s)
		}
		for _, o := range options {
			if o.Value == s && !o.Retired {
				return marshal(s)
			}
		}
		return nil, false
	}
	return nil, false
}

func marshal(v any) (json.RawMessage, bool) {
	b, err := json.Marshal(v)
	return b, err == nil
}

func previousString(previous json.RawMessage) string {
	var s string
	if len(previous) == 0 || json.Unmarshal(previous, &s) != nil {
		return ""
	}
	return s
}

// CheckOptions validates the options of a select field. Options that existed
// before and are missing from the request are kept as retired, so records
// that have them keep their value.
func CheckOptions(typ string, options, previous []Option) (json.RawMessage, error) {
	if typ != TypeSelect {
		return json.RawMessage("[]"), nil
	}
	out := make([]Option, 0, len(options)+len(previous))
	seen := map[string]bool{}
	active := 0
	for _, o := range options {
		if seen[o.Value] {
			return nil, apperr.ErrValidation.WithDetail("field", "options")
		}
		seen[o.Value] = true
		if !o.Retired {
			active++
		}
		out = append(out, o)
	}
	for _, o := range previous {
		if !seen[o.Value] {
			out = append(out, Option{Value: o.Value, Retired: true})
		}
	}
	if active == 0 {
		return nil, ErrOptionNeeded
	}
	return json.Marshal(out)
}

// NormalizeName compares names ignoring case and outer spaces.
func NormalizeName(name string) string { return strings.ToLower(strings.TrimSpace(name)) }
