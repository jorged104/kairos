// Package richtext validates the descriptions of spec 004 (RF-70, RF-71) and
// the pages of spec 005. A description is a TipTap/ProseMirror JSON document,
// never HTML: every node, mark and attribute must be in the allowlist, and the
// stored copy is rebuilt from the parsed tree so nothing outside it reaches the
// database. Pages admit more blocks than the descriptions of objectives.
package richtext

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"openscum/internal/apperr"
)

const (
	MaxBytes = 256 << 10
	MaxRunes = 50_000
	MaxDepth = 20
)

var ErrInvalid = apperr.New(http.StatusBadRequest, "invalid_rich_text", "La descripción tiene un formato no admitido")

type node struct {
	Type    string                     `json:"type"`
	Attrs   map[string]json.RawMessage `json:"attrs,omitempty"`
	Content []*node                    `json:"content,omitempty"`
	Marks   []*mark                    `json:"marks,omitempty"`
	Text    *string                    `json:"text,omitempty"`
}

type mark struct {
	Type  string                     `json:"type"`
	Attrs map[string]json.RawMessage `json:"attrs,omitempty"`
}

// attrRule validates one attribute value; ok=false rejects the document.
// keep=false drops the attribute from the stored copy.
type attrRule func(v json.RawMessage) (ok, keep bool)

var (
	headingLevel = intRange(1, 3)
	span         = intRange(1, 20)
	languageRe   = regexp.MustCompile(`^[a-z0-9+#-]{0,30}$`)
)

// nodeAttrs lists the allowed nodes and their attributes.
var nodeAttrs = map[string]map[string]attrRule{
	"doc":            {},
	"paragraph":      {},
	"heading":        {"level": headingLevel},
	"bulletList":     {},
	"orderedList":    {"start": intRange(1, 10_000), "type": oneOfOrNull("1", "a", "A", "i", "I")},
	"listItem":       {},
	"codeBlock":      {"language": stringMatchOrNull(languageRe)},
	"blockquote":     {},
	"hardBreak":      {},
	"horizontalRule": {},
	"table":          {},
	"tableRow":       {},
	"tableHeader":    {"colspan": span, "rowspan": span, "colwidth": colwidth},
	"tableCell":      {"colspan": span, "rowspan": span, "colwidth": colwidth},
	"text":           {},
}

// markAttrs lists the allowed marks. Link keeps only its href: target and rel
// are set by the viewer.
var markAttrs = map[string]map[string]attrRule{
	"bold":      {},
	"italic":    {},
	"strike":    {},
	"underline": {},
	"code":      {},
	"link": {
		"href":   safeHref,
		"target": dropString,
		"rel":    dropString,
		"class":  dropString,
		"title":  dropString,
	},
}

// pageNodeAttrs adds the blocks of a page (spec 005, RF-1 to RF-6) to nodeAttrs.
// The files and mentions are checked by the cleaner.
var pageNodeAttrs = func() map[string]map[string]attrRule {
	m := map[string]map[string]attrRule{
		"taskList": {},
		"taskItem": {"checked": boolean},
		// Toggles always open folded (RF-4): their state is never stored.
		"details":        {"open": dropBool},
		"detailsSummary": {},
		"detailsContent": {},
		"callout":        {},
		// The viewer builds src from fileId; the src of the editor is dropped.
		"image": {
			"fileId": uuidString, "alt": stringOrNull(200), "src": fileSrc,
			"title": dropString, "width": dropNumber, "height": dropNumber,
		},
		"fileAttachment": {"fileId": uuidString, "name": dropString, "size": dropNumber},
		// The label is resolved when reading (RF-27).
		"mention": {
			"kind": oneOf("user", "work_item", "milestone", "project"), "id": uuidString,
			"label": dropString, "mentionSuggestionChar": dropString,
		},
	}
	for k, v := range nodeAttrs {
		m[k] = v
	}
	return m
}()

// File is a file of the page's owner.
type File struct {
	Name string
	Size int64
	Kind string // image or file
}

// Mention is a reference of a page to a user or another element.
type Mention struct {
	Kind string
	ID   uuid.UUID
}

// Page is what CleanPage found in a page besides its text.
type Page struct {
	Files    []uuid.UUID
	Mentions []Mention
}

type cleaner struct {
	nodes map[string]map[string]attrRule
	files map[uuid.UUID]File
	page  Page
	seen  map[uuid.UUID]bool
}

// Clean validates raw and returns the document to store and its plain text.
// An empty or null document returns nil and "".
func Clean(raw json.RawMessage) (json.RawMessage, string, error) {
	return (&cleaner{nodes: nodeAttrs}).clean(raw)
}

// CleanPage validates a page (spec 005). files are the files of the page's
// owner: an image or attachment of any other file is rejected, and the stored
// name and size of an attachment come from its file. It also returns the
// files and mentions of the page.
func CleanPage(raw json.RawMessage, files map[uuid.UUID]File) (json.RawMessage, string, Page, error) {
	c := &cleaner{nodes: pageNodeAttrs, files: files, seen: map[uuid.UUID]bool{}}
	out, text, err := c.clean(raw)
	if err != nil {
		return nil, "", Page{}, err
	}
	return out, text, c.page, nil
}

func (c *cleaner) clean(raw json.RawMessage) (json.RawMessage, string, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil, "", nil
	}
	if len(raw) > MaxBytes {
		return nil, "", ErrInvalid.WithDetail("reason", "size")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var doc node
	if err := dec.Decode(&doc); err != nil || doc.Type != "doc" {
		return nil, "", ErrInvalid.WithDetail("reason", "structure")
	}
	var text strings.Builder
	if err := c.walk(&doc, 0, &text); err != nil {
		return nil, "", err
	}
	plain := strings.TrimSpace(text.String())
	if utf8.RuneCountInString(plain) > MaxRunes {
		return nil, "", ErrInvalid.WithDetail("reason", "length")
	}
	if plain == "" && !hasStructure(&doc) {
		return nil, "", nil
	}
	out, err := json.Marshal(&doc)
	if err != nil {
		return nil, "", ErrInvalid
	}
	return out, plain, nil
}

func (c *cleaner) walk(n *node, depth int, text *strings.Builder) error {
	if n == nil || depth > MaxDepth {
		return ErrInvalid.WithDetail("reason", "depth")
	}
	rules, ok := c.nodes[n.Type]
	if !ok || (depth > 0 && n.Type == "doc") {
		return ErrInvalid.WithDetail("reason", "node")
	}
	var err error
	if n.Attrs, err = checkAttrs(n.Attrs, rules); err != nil {
		return err
	}
	if err := c.refs(n, text); err != nil {
		return err
	}
	if n.Type == "text" {
		if n.Text == nil || *n.Text == "" || len(n.Content) > 0 {
			return ErrInvalid.WithDetail("reason", "text")
		}
		text.WriteString(*n.Text)
	} else if n.Text != nil || len(n.Marks) > 0 {
		return ErrInvalid.WithDetail("reason", "node")
	}
	for _, m := range n.Marks {
		if m == nil {
			return ErrInvalid.WithDetail("reason", "mark")
		}
		mr, ok := markAttrs[m.Type]
		if !ok {
			return ErrInvalid.WithDetail("reason", "mark")
		}
		if m.Attrs, err = checkAttrs(m.Attrs, mr); err != nil {
			return err
		}
		if m.Type == "link" && m.Attrs["href"] == nil {
			return ErrInvalid.WithDetail("reason", "link")
		}
	}
	for _, k := range n.Content {
		if err := c.walk(k, depth+1, text); err != nil {
			return err
		}
	}
	switch n.Type {
	case "paragraph", "heading", "codeBlock", "tableCell", "tableHeader", "hardBreak", "detailsSummary":
		text.WriteString("\n")
	}
	return nil
}

// refs checks the files and mentions of a page node. An attachment takes its
// name and size from its file, and the name goes to the searchable text (RF-46).
func (c *cleaner) refs(n *node, text *strings.Builder) error {
	switch n.Type {
	case "image", "fileAttachment":
		id, ok := attrUUID(n.Attrs, "fileId")
		f, mine := c.files[id]
		if !ok || !mine || (n.Type == "image") != (f.Kind == "image") {
			return ErrInvalid.WithDetail("reason", "file")
		}
		if n.Type == "fileAttachment" {
			name, _ := json.Marshal(f.Name)
			size, _ := json.Marshal(f.Size)
			n.Attrs["name"], n.Attrs["size"] = name, size
			text.WriteString(f.Name + "\n")
		}
		if !c.seen[id] {
			c.seen[id] = true
			c.page.Files = append(c.page.Files, id)
		}
	case "mention":
		id, ok := attrUUID(n.Attrs, "id")
		var kind string
		if !ok || json.Unmarshal(n.Attrs["kind"], &kind) != nil {
			return ErrInvalid.WithDetail("reason", "mention")
		}
		c.page.Mentions = append(c.page.Mentions, Mention{Kind: kind, ID: id})
	}
	return nil
}

func attrUUID(attrs map[string]json.RawMessage, key string) (uuid.UUID, bool) {
	var s string
	if json.Unmarshal(attrs[key], &s) != nil {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(s)
	return id, err == nil
}

// checkAttrs keeps the allowed attributes. Unknown attributes are dropped
// when null (editors send defaults) and reject the document otherwise.
func checkAttrs(attrs map[string]json.RawMessage, rules map[string]attrRule) (map[string]json.RawMessage, error) {
	var out map[string]json.RawMessage
	for k, v := range attrs {
		rule, ok := rules[k]
		if !ok {
			if isNull(v) {
				continue
			}
			return nil, ErrInvalid.WithDetail("reason", "attribute")
		}
		valid, keep := rule(v)
		if !valid {
			return nil, ErrInvalid.WithDetail("reason", "attribute")
		}
		if keep {
			if out == nil {
				out = map[string]json.RawMessage{}
			}
			out[k] = v
		}
	}
	return out, nil
}

func hasStructure(doc *node) bool {
	for _, c := range doc.Content {
		if c.Type != "paragraph" || len(c.Content) > 0 {
			return true
		}
	}
	return false
}

func isNull(v json.RawMessage) bool { return string(bytes.TrimSpace(v)) == "null" }

func intRange(lo, hi int) attrRule {
	return func(v json.RawMessage) (bool, bool) {
		var n int
		if json.Unmarshal(v, &n) != nil {
			return false, false
		}
		return n >= lo && n <= hi, true
	}
}

func oneOfOrNull(values ...string) attrRule {
	return func(v json.RawMessage) (bool, bool) {
		if isNull(v) {
			return true, false
		}
		var s string
		if json.Unmarshal(v, &s) != nil {
			return false, false
		}
		for _, x := range values {
			if s == x {
				return true, true
			}
		}
		return false, false
	}
}

func stringMatchOrNull(re *regexp.Regexp) attrRule {
	return func(v json.RawMessage) (bool, bool) {
		if isNull(v) {
			return true, false
		}
		var s string
		if json.Unmarshal(v, &s) != nil {
			return false, false
		}
		return re.MatchString(s), true
	}
}

func colwidth(v json.RawMessage) (bool, bool) {
	if isNull(v) {
		return true, false
	}
	var ws []int
	if json.Unmarshal(v, &ws) != nil || len(ws) > 20 {
		return false, false
	}
	for _, w := range ws {
		if w < 0 || w > 5000 {
			return false, false
		}
	}
	return true, true
}

func oneOf(values ...string) attrRule {
	return func(v json.RawMessage) (bool, bool) {
		var s string
		if json.Unmarshal(v, &s) != nil {
			return false, false
		}
		for _, x := range values {
			if s == x {
				return true, true
			}
		}
		return false, false
	}
}

func boolean(v json.RawMessage) (bool, bool) {
	var b bool
	return json.Unmarshal(v, &b) == nil, true
}

func dropBool(v json.RawMessage) (bool, bool) {
	var b *bool
	return json.Unmarshal(v, &b) == nil, false
}

func dropNumber(v json.RawMessage) (bool, bool) {
	var f *float64
	return json.Unmarshal(v, &f) == nil, false
}

// uuidString accepts a uuid in its canonical form only.
func uuidString(v json.RawMessage) (bool, bool) {
	var s string
	if json.Unmarshal(v, &s) != nil || len(s) != 36 {
		return false, false
	}
	_, err := uuid.Parse(s)
	return err == nil, true
}

func stringOrNull(max int) attrRule {
	return func(v json.RawMessage) (bool, bool) {
		if isNull(v) {
			return true, false
		}
		var s string
		return json.Unmarshal(v, &s) == nil && utf8.RuneCountInString(s) <= max, true
	}
}

var fileSrcRe = regexp.MustCompile(`^/api/files/[0-9a-f-]{36}$`)

// fileSrc accepts only the URL the viewer builds for a file, and drops it.
func fileSrc(v json.RawMessage) (bool, bool) {
	if isNull(v) {
		return true, false
	}
	var s string
	return json.Unmarshal(v, &s) == nil && fileSrcRe.MatchString(s), false
}

// dropString accepts a short string or null and never stores it.
func dropString(v json.RawMessage) (bool, bool) {
	if isNull(v) {
		return true, false
	}
	var s string
	return json.Unmarshal(v, &s) == nil && len(s) <= 200, false
}

// safeHref admits http, https and mailto links only (RF-71).
func safeHref(v json.RawMessage) (bool, bool) {
	var s string
	if json.Unmarshal(v, &s) != nil || s == "" || len(s) > 2000 {
		return false, false
	}
	u, err := url.Parse(s)
	if err != nil {
		return false, false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return u.Host != "", true
	case "mailto":
		return u.Opaque != "", true
	}
	return false, false
}

// Changed reports whether two stored documents differ in content; the
// database may return the same JSON with other spacing or key order.
func Changed(a, b json.RawMessage) bool {
	var x, y any
	if len(bytes.TrimSpace(a)) > 0 {
		_ = json.Unmarshal(a, &x)
	}
	if len(bytes.TrimSpace(b)) > 0 {
		_ = json.Unmarshal(b, &y)
	}
	return !reflect.DeepEqual(x, y)
}

// Mentions lists the mentions of a stored page, to resolve them when reading
// (RF-27). The page was validated when saved, so a malformed one yields none.
func Mentions(doc json.RawMessage) []Mention {
	var root node
	if len(bytes.TrimSpace(doc)) == 0 || json.Unmarshal(doc, &root) != nil {
		return nil
	}
	var out []Mention
	var walk func(n *node)
	walk = func(n *node) {
		if n == nil {
			return
		}
		if n.Type == "mention" {
			id, ok := attrUUID(n.Attrs, "id")
			var kind string
			if ok && json.Unmarshal(n.Attrs["kind"], &kind) == nil {
				out = append(out, Mention{Kind: kind, ID: id})
			}
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(&root)
	return out
}
