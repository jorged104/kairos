package richtext

import (
	"errors"
	"strings"
	"testing"

	"openscum/internal/apperr"
)

func TestCleanAcceptsTheEditorOutput(t *testing.T) {
	doc := `{"type":"doc","content":[
		{"type":"heading","attrs":{"level":2,"id":null},"content":[{"type":"text","text":"Alcance"}]},
		{"type":"paragraph","content":[
			{"type":"text","marks":[{"type":"bold"}],"text":"Hola "},
			{"type":"text","marks":[{"type":"link","attrs":{"href":"https://example.com/a?b=1","target":"_blank","rel":"noopener noreferrer nofollow","class":null}}],"text":"enlace"}
		]},
		{"type":"orderedList","attrs":{"start":1,"type":null},"content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"uno"}]}]}]},
		{"type":"codeBlock","attrs":{"language":"go"},"content":[{"type":"text","text":"x := 1"}]},
		{"type":"table","content":[{"type":"tableRow","content":[
			{"type":"tableHeader","attrs":{"colspan":1,"rowspan":1,"colwidth":null},"content":[{"type":"paragraph","content":[{"type":"text","text":"A"}]}]},
			{"type":"tableCell","attrs":{"colspan":1,"rowspan":1,"colwidth":[120]},"content":[{"type":"paragraph"}]}
		]}]},
		{"type":"paragraph","content":[{"type":"text","marks":[{"type":"link","attrs":{"href":"mailto:ana@example.com"}}],"text":"correo"}]}
	]}`
	out, text, err := Clean([]byte(doc))
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	for _, want := range []string{"Alcance", "Hola enlace", "uno", "x := 1", "correo"} {
		if !strings.Contains(text, want) {
			t.Errorf("text %q lacks %q", text, want)
		}
	}
	s := string(out)
	for _, gone := range []string{"_blank", "noopener", `"id"`, `"class"`} {
		if strings.Contains(s, gone) {
			t.Errorf("stored document keeps %s: %s", gone, s)
		}
	}
	if !strings.Contains(s, `"href":"https://example.com/a?b=1"`) {
		t.Errorf("stored document lost the href: %s", s)
	}
}

func TestCleanEmpty(t *testing.T) {
	for _, raw := range []string{``, `null`, ` `, `{"type":"doc"}`, `{"type":"doc","content":[{"type":"paragraph"}]}`} {
		out, text, err := Clean([]byte(raw))
		if err != nil || out != nil || text != "" {
			t.Errorf("Clean(%q) = %s, %q, %v; want empty", raw, out, text, err)
		}
	}
}

func TestCleanRejects(t *testing.T) {
	long := strings.Repeat("a", MaxRunes+1)
	deep := strings.Repeat(`{"type":"blockquote","content":[`, MaxDepth+1) + `{"type":"paragraph"}` + strings.Repeat(`]}`, MaxDepth+1)
	cases := map[string]string{
		"html string":          `"<script>alert(1)</script>"`,
		"not a doc":            `{"type":"paragraph"}`,
		"unknown node":         `{"type":"doc","content":[{"type":"iframe","attrs":{"src":"https://x"}}]}`,
		"html node":            `{"type":"doc","content":[{"type":"html","text":"<img onerror=x>"}]}`,
		"unknown mark":         `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"a","marks":[{"type":"textStyle","attrs":{"style":"x"}}]}]}]}`,
		"javascript link":      `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"a","marks":[{"type":"link","attrs":{"href":"javascript:alert(1)"}}]}]}]}`,
		"spaced javascript":    `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"a","marks":[{"type":"link","attrs":{"href":" javascript:alert(1)"}}]}]}]}`,
		"data link":            `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"a","marks":[{"type":"link","attrs":{"href":"data:text/html,x"}}]}]}]}`,
		"link without href":    `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"a","marks":[{"type":"link"}]}]}]}`,
		"extra attribute":      `{"type":"doc","content":[{"type":"paragraph","attrs":{"style":"color:red"}}]}`,
		"extra key":            `{"type":"doc","html":"<b>x</b>"}`,
		"heading level 4":      `{"type":"doc","content":[{"type":"heading","attrs":{"level":4}}]}`,
		"code language":        `{"type":"doc","content":[{"type":"codeBlock","attrs":{"language":"\"><script>"}}]}`,
		"empty text":           `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":""}]}]}`,
		"text on a paragraph":  `{"type":"doc","content":[{"type":"paragraph","text":"x"}]}`,
		"nested doc":           `{"type":"doc","content":[{"type":"doc"}]}`,
		"null child":           `{"type":"doc","content":[null]}`,
		"too long":             `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"` + long + `"}]}]}`,
		"too deep":             `{"type":"doc","content":[` + deep + `]}`,
		"too big":              `{"type":"doc","content":[` + strings.Repeat(`{"type":"horizontalRule"},`, MaxBytes/25) + `{"type":"horizontalRule"}]}`,
		"colspan out of range": `{"type":"doc","content":[{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableCell","attrs":{"colspan":99}}]}]}]}`,
	}
	for name, raw := range cases {
		_, _, err := Clean([]byte(raw))
		var e *apperr.Error
		if !errors.As(err, &e) || e.Code != "invalid_rich_text" {
			t.Errorf("%s: err = %v, want invalid_rich_text", name, err)
		}
	}
}
