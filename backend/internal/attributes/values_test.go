package attributes

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"openscum/internal/apperr"
)

func TestNormalizeByType(t *testing.T) {
	sel := Attribute{ID: uuid.New(), Type: TypeSelect, Options: []Option{{Value: "a"}, {Value: "old", Retired: true}}}
	cases := []struct {
		attr     Attribute
		raw      string
		previous string
		want     string // "" = cleared, "ERR" = invalid
	}{
		{Attribute{Type: TypeText}, `"  hola "`, "", `"hola"`},
		{Attribute{Type: TypeText}, `"   "`, "", ""},
		{Attribute{Type: TypeText}, `12`, "", "ERR"},
		{Attribute{Type: TypeNumber}, `12.5`, "", `12.5`},
		{Attribute{Type: TypeNumber}, `"12"`, "", "ERR"},
		{Attribute{Type: TypeDate}, `"2026-10-22"`, "", `"2026-10-22"`},
		{Attribute{Type: TypeDate}, `"abc"`, "", "ERR"},
		{Attribute{Type: TypeDate}, `"2026-02-30"`, "", "ERR"},
		{Attribute{Type: TypeBoolean}, `true`, "", `true`},
		{Attribute{Type: TypeBoolean}, `"true"`, "", "ERR"},
		{sel, `"a"`, "", `"a"`},
		{sel, `"zzz"`, "", "ERR"},
		{sel, `"old"`, "", "ERR"},
		{sel, `"old"`, `"old"`, `"old"`}, // keeps a retired option it already had (RF-107)
		{Attribute{Type: TypeNumber}, `null`, "", ""},
	}
	for _, c := range cases {
		got, err := Normalize(c.attr, json.RawMessage(c.raw), json.RawMessage(c.previous))
		switch {
		case c.want == "ERR":
			if !errors.Is(err, ErrInvalidValue) && (err == nil || err.(*apperr.Error).Code != ErrInvalidValue.Code) {
				t.Errorf("%s %s: err = %v, want invalid", c.attr.Type, c.raw, err)
			}
		case err != nil:
			t.Errorf("%s %s: %v", c.attr.Type, c.raw, err)
		case string(got) != c.want:
			t.Errorf("%s %s = %s, want %q", c.attr.Type, c.raw, got, c.want)
		}
	}
}

func TestCheckOptions(t *testing.T) {
	if _, err := checkOptions(TypeSelect, nil, nil); !errors.Is(err, errOptionNeeded) {
		t.Fatalf("select without options: %v", err)
	}
	if _, err := checkOptions(TypeSelect, []Option{{Value: "a", Retired: true}}, nil); !errors.Is(err, errOptionNeeded) {
		t.Fatalf("select with only retired options: %v", err)
	}
	raw, err := checkOptions(TypeSelect, []Option{{Value: "b"}}, []Option{{Value: "a"}, {Value: "b"}})
	if err != nil {
		t.Fatal(err)
	}
	var got []Option
	_ = json.Unmarshal(raw, &got)
	if len(got) != 2 || got[1] != (Option{Value: "a", Retired: true}) {
		t.Fatalf("options = %+v, want the missing one kept as retired", got)
	}
	if raw, _ := checkOptions(TypeText, []Option{{Value: "x"}}, nil); string(raw) != "[]" {
		t.Fatalf("non-select options = %s", raw)
	}
}
