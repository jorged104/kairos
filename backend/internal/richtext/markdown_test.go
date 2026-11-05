package richtext

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// What an AI client writes is a document the page cleaner accepts, and it
// reads back the same (spec 016, RF-34 to RF-36).
func TestMarkdownRoundTrip(t *testing.T) {
	md := strings.Join([]string{
		"# Plan de la semana",
		"",
		"Texto con **negrita**, *cursiva*, ~~tachado~~, `código` y [un enlace](https://example.com).",
		"Segunda línea del párrafo.",
		"",
		"- uno",
		"- dos",
		"  - dos punto uno",
		"",
		"3. tres",
		"4. cuatro",
		"",
		"- [ ] pendiente",
		"- [x] hecha",
		"",
		"> una cita",
		"",
		"```go",
		"fmt.Println(\"hola\")",
		"```",
		"",
		"---",
		"",
		"| A | B |",
		"| --- | --- |",
		"| 1 | 2 |",
	}, "\n")
	doc := FromMarkdown(md)
	if _, _, _, err := CleanPage(doc, nil); err != nil {
		t.Fatalf("CleanPage: %v\n%s", err, doc)
	}
	if _, _, _, err := CleanComment(doc, nil); err != nil {
		t.Fatalf("CleanComment: %v", err)
	}
	if got := ToMarkdown(doc, nil); got != md {
		t.Fatalf("round trip:\n%s\n--- want ---\n%s", got, md)
	}
	for _, want := range []string{`"taskList"`, `"checked":true`, `"orderedList"`, `"start":3`, `"language":"go"`, `"hardBreak"`, `"tableHeader"`} {
		if !strings.Contains(string(doc), want) {
			t.Errorf("missing %s in %s", want, doc)
		}
	}
}

func TestMarkdownInline(t *testing.T) {
	cases := map[string]string{
		"snake_case_name":             `[{"type":"text","text":"snake_case_name"}]`,
		"precio 2*3*4":                `[{"type":"text","text":"precio 2*3*4"}]`,
		"ver https://example.com/x.":  `[{"type":"text","text":"ver "},{"type":"text","marks":[{"type":"link","attrs":{"href":"https://example.com/x"}}],"text":"https://example.com/x"},{"type":"text","text":"."}]`,
		"[malo](javascript:alert(1))": `[{"type":"text","text":"[malo](javascript:alert(1))"}]`,
		"**negrita *y cursiva***":     `[{"type":"text","marks":[{"type":"bold"}],"text":"negrita "},{"type":"text","marks":[{"type":"bold"},{"type":"italic"}],"text":"y cursiva"}]`,
		`\*literal\*`:                 `[{"type":"text","text":"*literal*"}]`,
	}
	for in, want := range cases {
		got, _ := json.Marshal(mdInline(in, nil))
		if string(got) != want {
			t.Errorf("%q:\n got %s\nwant %s", in, got, want)
		}
	}
}

func TestMarkdownEdges(t *testing.T) {
	if got := string(FromMarkdown("")); got != `{"type":"doc","content":[{"type":"paragraph"}]}` {
		t.Errorf("empty: %s", got)
	}
	// HTML is text, never markup (ADR 0003).
	doc := FromMarkdown("<script>alert(1)</script>")
	if _, text, _, err := CleanPage(doc, nil); err != nil || text != "<script>alert(1)</script>" {
		t.Errorf("html: %v %q", err, text)
	}
	// Mentions and files read as text.
	id := uuid.New()
	stored := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"hola "},` +
		`{"type":"mention","attrs":{"kind":"user","id":"` + id.String() + `"}}]},{"type":"image","attrs":{"fileId":"` + uuid.NewString() + `","alt":"plano"}}]}`
	if got := ToMarkdown(json.RawMessage(stored), map[string]string{id.String(): "Ana"}); got != "hola @Ana\n\n[imagen: plano]" {
		t.Errorf("mentions: %q", got)
	}
}
