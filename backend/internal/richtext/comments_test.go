package richtext

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

var thread = uuid.MustParse("44444444-4444-4444-8444-444444444444")

func TestCommentMarkOnlyInPages(t *testing.T) {
	doc := `{"type":"doc","content":[{"type":"paragraph","content":[
		{"type":"text","text":"Hola "},
		{"type":"text","text":"mundo","marks":[{"type":"bold"},{"type":"comment","attrs":{"threadId":"` + thread.String() + `"}}]}]}]}`
	_, _, page, err := CleanPage([]byte(doc), nil)
	if err != nil {
		t.Fatalf("page with an anchor: %v", err)
	}
	if len(page.Threads) != 1 || page.Threads[0] != thread {
		t.Fatalf("threads = %v", page.Threads)
	}
	if _, _, err := Clean([]byte(doc)); err == nil {
		t.Fatal("a description of an objective accepts anchors")
	}
	if _, _, _, err := CleanComment([]byte(doc), nil); err == nil {
		t.Fatal("a comment accepts anchors")
	}
	bad := strings.Replace(doc, thread.String(), "no", 1)
	if _, _, _, err := CleanPage([]byte(bad), nil); err == nil {
		t.Fatal("an anchor without a uuid is accepted")
	}
	stored, _, _, _ := CleanPage([]byte(doc), nil)
	if got := Threads(stored); !got[thread] || len(got) != 1 {
		t.Fatalf("Threads = %v", got)
	}
}

func TestCommentBodies(t *testing.T) {
	doc := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Mira "},
		{"type":"mention","attrs":{"kind":"user","id":"` + user.String() + `"}}]},
		{"type":"image","attrs":{"fileId":"` + img.String() + `"}}]}`
	_, text, page, err := CleanComment([]byte(doc), ownerFiles())
	if err != nil || text != "Mira" || len(page.Files) != 1 || len(page.Mentions) != 1 {
		t.Fatalf("comment = %q %+v %v", text, page, err)
	}
	if _, _, _, err := CleanComment([]byte(doc), nil); err == nil {
		t.Fatal("a comment with someone else's image is accepted")
	}
}

func TestSameButAnchors(t *testing.T) {
	plain := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Hola mundo","marks":[{"type":"bold"}]}]}]}`
	anchored := `{"type":"doc","content":[{"type":"paragraph","content":[
		{"type":"text","text":"Hola ","marks":[{"type":"bold"}]},
		{"type":"text","text":"mundo","marks":[{"type":"bold"},{"type":"comment","attrs":{"threadId":"` + thread.String() + `"}}]}]}]}`
	a, _, _, _ := CleanPage([]byte(plain), nil)
	b, _, _, _ := CleanPage([]byte(anchored), nil)
	if !SameButAnchors(a, b) {
		t.Fatal("adding an anchor counts as a change")
	}
	changed := strings.Replace(anchored, "mundo", "mundos", 1)
	c, _, _, _ := CleanPage([]byte(changed), nil)
	if SameButAnchors(a, c) {
		t.Fatal("a text change passes as an anchor")
	}
	unbold := strings.Replace(anchored, `{"type":"bold"},{"type":"comment"`, `{"type":"comment"`, 1)
	d, _, _, _ := CleanPage([]byte(unbold), nil)
	if SameButAnchors(a, d) {
		t.Fatal("a format change passes as an anchor")
	}
	if !SameButAnchors(nil, nil) {
		t.Fatal("two empty pages differ")
	}
}
