package richtext

import (
	"bytes"
	"encoding/json"
	"reflect"

	"github.com/google/uuid"
)

// CommentMark anchors a comment thread to a piece of a page's text (spec 006,
// RF-45). Its threadId is the id of the thread's root comment.
const CommentMark = "comment"

// pageMarkAttrs adds the comment anchor to the marks of a page.
var pageMarkAttrs = func() map[string]map[string]attrRule {
	m := map[string]map[string]attrRule{CommentMark: {"threadId": uuidString}}
	for k, v := range markAttrs {
		m[k] = v
	}
	return m
}()

// CleanComment validates the body of a comment (spec 006, RF-26): the blocks,
// files and mentions of a page, without anchors. files are the files of the
// comments of its work item.
func CleanComment(raw json.RawMessage, files map[uuid.UUID]File) (json.RawMessage, string, Page, error) {
	c := &cleaner{nodes: pageNodeAttrs, marks: markAttrs, files: files, seen: map[uuid.UUID]bool{}}
	out, text, err := c.clean(raw)
	if err != nil {
		return nil, "", Page{}, err
	}
	return out, text, c.page, nil
}

// Threads lists the threads anchored in a stored page, to tell which kept
// their text (RF-51). The page was validated when saved.
func Threads(doc json.RawMessage) map[uuid.UUID]bool {
	out := map[uuid.UUID]bool{}
	var root node
	if len(bytes.TrimSpace(doc)) == 0 || json.Unmarshal(doc, &root) != nil {
		return out
	}
	var walk func(n *node)
	walk = func(n *node) {
		if n == nil {
			return
		}
		for _, m := range n.Marks {
			if m != nil && m.Type == CommentMark {
				if id, ok := attrUUID(m.Attrs, "threadId"); ok {
					out[id] = true
				}
			}
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(&root)
	return out
}

// SameButAnchors says whether two stored pages differ only in their comment
// anchors, so someone who can't edit the page may still anchor a comment in
// it (RF-46).
func SameButAnchors(a, b json.RawMessage) bool {
	x, okA := stripped(a)
	y, okB := stripped(b)
	return okA && okB && reflect.DeepEqual(x, y)
}

// stripped is a page without comment marks, its adjacent texts with the same
// marks joined: an anchor splits a text in two.
func stripped(doc json.RawMessage) (any, bool) {
	if len(bytes.TrimSpace(doc)) == 0 || string(bytes.TrimSpace(doc)) == "null" {
		return nil, true
	}
	var root node
	if json.Unmarshal(doc, &root) != nil {
		return nil, false
	}
	var strip func(n *node)
	strip = func(n *node) {
		marks := n.Marks[:0]
		for _, m := range n.Marks {
			if m != nil && m.Type != CommentMark {
				marks = append(marks, m)
			}
		}
		n.Marks = marks
		joined := make([]*node, 0, len(n.Content))
		for _, c := range n.Content {
			if c == nil {
				continue
			}
			strip(c)
			if last := len(joined) - 1; last >= 0 && c.Type == "text" && joined[last].Type == "text" &&
				c.Text != nil && joined[last].Text != nil && reflect.DeepEqual(marksOf(c), marksOf(joined[last])) {
				t := *joined[last].Text + *c.Text
				joined[last].Text = &t
				continue
			}
			joined = append(joined, c)
		}
		n.Content = joined
	}
	strip(&root)
	out, err := json.Marshal(&root)
	if err != nil {
		return nil, false
	}
	var v any
	if err := json.Unmarshal(out, &v); err != nil {
		return nil, false
	}
	return v, true
}

// marksOf compares marks by value, an empty list as none.
func marksOf(n *node) any {
	if len(n.Marks) == 0 {
		return nil
	}
	raw, _ := json.Marshal(n.Marks)
	var v any
	_ = json.Unmarshal(raw, &v)
	return v
}
