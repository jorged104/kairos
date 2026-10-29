package richtext

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"openscum/internal/apperr"
)

var (
	img  = uuid.MustParse("11111111-1111-4111-8111-111111111111")
	pdf  = uuid.MustParse("22222222-2222-4222-8222-222222222222")
	user = uuid.MustParse("33333333-3333-4333-8333-333333333333")
)

func ownerFiles() map[uuid.UUID]File {
	return map[uuid.UUID]File{
		img: {Name: "login.png", Size: 2048, Kind: "image"},
		pdf: {Name: "Contrato IdP.pdf", Size: 1200000, Kind: "file"},
	}
}

func TestCleanPageAcceptsTheEditorOutput(t *testing.T) {
	doc := `{"type":"doc","content":[
		{"type":"taskList","content":[{"type":"taskItem","attrs":{"checked":true},"content":[{"type":"paragraph","content":[{"type":"text","text":"hecho"}]}]}]},
		{"type":"details","attrs":{"open":true},"content":[
			{"type":"detailsSummary","content":[{"type":"text","text":"Notas"}]},
			{"type":"detailsContent","content":[{"type":"details","content":[
				{"type":"detailsSummary","content":[{"type":"text","text":"Dentro"}]},
				{"type":"detailsContent","content":[{"type":"paragraph"}]}
			]}]}
		]},
		{"type":"callout","content":[{"type":"paragraph","content":[{"type":"text","text":"Ojo"}]}]},
		{"type":"image","attrs":{"fileId":"` + img.String() + `","src":"/api/files/` + img.String() + `","alt":"captura","title":null,"width":null,"height":null}},
		{"type":"fileAttachment","attrs":{"fileId":"` + pdf.String() + `","name":"otro.exe","size":1}},
		{"type":"fileAttachment","attrs":{"fileId":"` + pdf.String() + `"}},
		{"type":"paragraph","content":[
			{"type":"text","text":"Revisa "},
			{"type":"mention","attrs":{"kind":"user","id":"` + user.String() + `","label":"Ana","mentionSuggestionChar":"@"}}
		]},
		{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableCell","content":[
			{"type":"taskList","content":[{"type":"taskItem","attrs":{"checked":false},"content":[{"type":"paragraph"}]}]}
		]}]}]}
	]}`
	out, text, page, err := CleanPage([]byte(doc), ownerFiles())
	if err != nil {
		t.Fatalf("CleanPage: %v", err)
	}
	s := string(out)
	for _, gone := range []string{`"open"`, `"src"`, `"label"`, `otro.exe`, `"size":1}`, `mentionSuggestionChar`} {
		if strings.Contains(s, gone) {
			t.Errorf("stored page keeps %s: %s", gone, s)
		}
	}
	for _, want := range []string{`"checked":true`, `"alt":"captura"`, `"name":"Contrato IdP.pdf"`, `"size":1200000`, `"kind":"user"`} {
		if !strings.Contains(s, want) {
			t.Errorf("stored page lacks %s: %s", want, s)
		}
	}
	// Searchable text with the new blocks and the attachment's name (RF-46).
	for _, want := range []string{"hecho", "Notas", "Dentro", "Ojo", "Contrato IdP.pdf", "Revisa"} {
		if !strings.Contains(text, want) {
			t.Errorf("text %q lacks %q", text, want)
		}
	}
	if len(page.Files) != 2 || page.Files[0] != img || page.Files[1] != pdf {
		t.Errorf("files = %v, want each once", page.Files)
	}
	if len(page.Mentions) != 1 || page.Mentions[0] != (Mention{Kind: "user", ID: user}) {
		t.Errorf("mentions = %v", page.Mentions)
	}
}

func TestCleanPageRejects(t *testing.T) {
	other := uuid.New().String()
	deep := strings.Repeat(`{"type":"details","content":[{"type":"detailsContent","content":[`, MaxDepth/2) +
		`{"type":"paragraph"}` + strings.Repeat(`]}]}`, MaxDepth/2)
	cases := map[string]string{
		"file of another owner": `{"type":"doc","content":[{"type":"image","attrs":{"fileId":"` + other + `"}}]}`,
		"image without file":    `{"type":"doc","content":[{"type":"image","attrs":{"src":"/api/files/` + img.String() + `"}}]}`,
		"outside src":           `{"type":"doc","content":[{"type":"image","attrs":{"fileId":"` + img.String() + `","src":"https://evil.example/x.png"}}]}`,
		"attachment as image":   `{"type":"doc","content":[{"type":"image","attrs":{"fileId":"` + pdf.String() + `"}}]}`,
		"image as attachment":   `{"type":"doc","content":[{"type":"fileAttachment","attrs":{"fileId":"` + img.String() + `"}}]}`,
		"bad file id":           `{"type":"doc","content":[{"type":"fileAttachment","attrs":{"fileId":"x"}}]}`,
		"mention kind":          `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"mention","attrs":{"kind":"team","id":"` + user.String() + `"}}]}]}`,
		"mention without id":    `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"mention","attrs":{"kind":"user"}}]}]}`,
		"checked not bool":      `{"type":"doc","content":[{"type":"taskList","content":[{"type":"taskItem","attrs":{"checked":"yes"}}]}]}`,
		"callout style":         `{"type":"doc","content":[{"type":"callout","attrs":{"style":"x"}}]}`,
		"21 levels":             `{"type":"doc","content":[` + deep + `]}`,
	}
	for name, raw := range cases {
		_, _, _, err := CleanPage([]byte(raw), ownerFiles())
		var e *apperr.Error
		if !errors.As(err, &e) || e.Code != "invalid_rich_text" {
			t.Errorf("%s: err = %v, want invalid_rich_text", name, err)
		}
	}
}

// Objectives keep the descriptions of spec 004: the page blocks are not theirs.
func TestCleanRejectsPageBlocks(t *testing.T) {
	for _, raw := range []string{
		`{"type":"doc","content":[{"type":"taskList","content":[]}]}`,
		`{"type":"doc","content":[{"type":"callout","content":[]}]}`,
		`{"type":"doc","content":[{"type":"image","attrs":{"fileId":"` + img.String() + `"}}]}`,
	} {
		_, _, err := Clean([]byte(raw))
		var e *apperr.Error
		if !errors.As(err, &e) || e.Code != "invalid_rich_text" {
			t.Errorf("Clean(%s) = %v, want invalid_rich_text", raw, err)
		}
	}
}
