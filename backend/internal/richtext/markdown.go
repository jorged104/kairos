package richtext

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The Markdown that AI clients read and write (spec 016, RF-34 to RF-36). It
// is a small, forgiving subset: paragraphs, headings, lists (task lists too),
// code, quotes, rules and tables; bold, italic, strike, code and links. What
// FromMarkdown builds still goes through Clean, CleanPage or CleanComment.

var (
	mdFence    = regexp.MustCompile("^\\s{0,3}(```+|~~~+)\\s*([A-Za-z0-9+#-]*)")
	mdHeading  = regexp.MustCompile(`^\s{0,3}(#{1,6})\s+(.*?)\s*#*\s*$`)
	mdRule     = regexp.MustCompile(`^\s{0,3}([-*_])(\s*[-*_]){2,}\s*$`)
	mdItem     = regexp.MustCompile(`^(\s*)([-*+]|\d{1,9}[.)])\s+(.*)$`)
	mdTask     = regexp.MustCompile(`^\[([ xX])\]\s+(.*)$`)
	mdTableSep = regexp.MustCompile(`^\s*\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?\s*$`)
	mdURL      = regexp.MustCompile(`^https?://[^\s<>()\[\]]+[^\s<>()\[\].,;:!?'"]`)
)

// FromMarkdown builds the TipTap document of a Markdown text.
func FromMarkdown(md string) json.RawMessage {
	md = strings.ReplaceAll(strings.ReplaceAll(md, "\r\n", "\n"), "\t", "    ")
	content := mdBlocks(strings.Split(md, "\n"))
	if len(content) == 0 {
		content = []*node{{Type: "paragraph"}}
	}
	out, _ := json.Marshal(&node{Type: "doc", Content: content})
	return out
}

func mdBlocks(lines []string) []*node {
	var out []*node
	for i := 0; i < len(lines); {
		line := lines[i]
		switch {
		case strings.TrimSpace(line) == "":
			i++
		case mdFence.MatchString(line):
			m := mdFence.FindStringSubmatch(line)
			var code []string
			i++
			for i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), m[1]) {
				code = append(code, lines[i])
				i++
			}
			i++
			n := &node{Type: "codeBlock", Attrs: map[string]json.RawMessage{"language": jsonValue(nil)}}
			if lang := strings.ToLower(m[2]); lang != "" && languageRe.MatchString(lang) {
				n.Attrs["language"] = jsonValue(lang)
			}
			if text := strings.Join(code, "\n"); text != "" {
				n.Content = []*node{textNode(text, nil)}
			}
			out = append(out, n)
		case mdHeading.MatchString(line):
			m := mdHeading.FindStringSubmatch(line)
			out = append(out, &node{Type: "heading", Attrs: map[string]json.RawMessage{"level": jsonValue(min(len(m[1]), 3))}, Content: mdInline(m[2], nil)})
			i++
		case mdRule.MatchString(line):
			out = append(out, &node{Type: "horizontalRule"})
			i++
		case strings.HasPrefix(strings.TrimSpace(line), ">"):
			var inner []string
			for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), ">") {
				s := strings.TrimPrefix(strings.TrimSpace(lines[i]), ">")
				inner = append(inner, strings.TrimPrefix(s, " "))
				i++
			}
			if content := mdBlocks(inner); len(content) > 0 {
				out = append(out, &node{Type: "blockquote", Content: content})
			}
		case mdItem.MatchString(line):
			var n *node
			n, i = mdList(lines, i)
			out = append(out, n)
		case i+1 < len(lines) && strings.Contains(line, "|") && mdTableSep.MatchString(lines[i+1]) && strings.Contains(lines[i+1], "-"):
			var n *node
			n, i = mdTable(lines, i)
			out = append(out, n)
		default:
			var para []string
			for i < len(lines) && strings.TrimSpace(lines[i]) != "" && (len(para) == 0 || !startsBlock(lines[i])) {
				para = append(para, strings.TrimSpace(lines[i]))
				i++
			}
			var content []*node
			for k, l := range para {
				if k > 0 {
					content = append(content, &node{Type: "hardBreak"})
				}
				content = append(content, mdInline(l, nil)...)
			}
			out = append(out, &node{Type: "paragraph", Content: content})
		}
	}
	return out
}

func startsBlock(line string) bool {
	return mdFence.MatchString(line) || mdHeading.MatchString(line) || mdRule.MatchString(line) ||
		strings.HasPrefix(strings.TrimSpace(line), ">") || mdItem.MatchString(line)
}

func indentOf(line string) int { return len(line) - len(strings.TrimLeft(line, " ")) }

func ordered(marker string) bool { return marker[0] >= '0' && marker[0] <= '9' }

// mdList reads the list that starts at lines[i]: its items are the following
// markers of the same kind and indentation, and every more indented line
// belongs to the item above it.
func mdList(lines []string, i int) (*node, int) {
	first := mdItem.FindStringSubmatch(lines[i])
	indent, isOrdered := len(first[1]), ordered(first[2])
	type item struct {
		lines []string
		task  *bool
	}
	var items []item
	for i < len(lines) {
		m := mdItem.FindStringSubmatch(lines[i])
		if m == nil || len(m[1]) != indent || ordered(m[2]) != isOrdered {
			break
		}
		width := len(m[1]) + len(m[2]) + 1
		it := item{lines: []string{m[3]}}
		if !isOrdered {
			if t := mdTask.FindStringSubmatch(m[3]); t != nil {
				checked := t[1] != " "
				it.task, it.lines[0] = &checked, t[2]
			}
		}
		i++
		for i < len(lines) {
			l := lines[i]
			if strings.TrimSpace(l) == "" {
				if i+1 < len(lines) && strings.TrimSpace(lines[i+1]) != "" && indentOf(lines[i+1]) > indent {
					it.lines = append(it.lines, "")
					i++
					continue
				}
				break
			}
			if indentOf(l) <= indent {
				break
			}
			it.lines = append(it.lines, l[min(width, indentOf(l)):])
			i++
		}
		items = append(items, it)
	}

	tasks := !isOrdered
	for _, it := range items {
		tasks = tasks && it.task != nil
	}
	list := &node{Type: "bulletList"}
	if isOrdered {
		list.Type = "orderedList"
		if start, _ := strconv.Atoi(strings.TrimRight(first[2], ".)")); start > 1 {
			list.Attrs = map[string]json.RawMessage{"start": jsonValue(min(start, 10_000))}
		}
	}
	if tasks {
		list.Type = "taskList"
	}
	for _, it := range items {
		if it.task != nil && !tasks {
			mark := "[ ] "
			if *it.task {
				mark = "[x] "
			}
			it.lines[0] = mark + it.lines[0]
		}
		content := mdBlocks(it.lines)
		if len(content) == 0 || content[0].Type != "paragraph" {
			content = append([]*node{{Type: "paragraph"}}, content...)
		}
		li := &node{Type: "listItem", Content: content}
		if tasks {
			li.Type, li.Attrs = "taskItem", map[string]json.RawMessage{"checked": jsonValue(*it.task)}
		}
		list.Content = append(list.Content, li)
	}
	return list, i
}

func mdTable(lines []string, i int) (*node, int) {
	cells := func(line string) []string {
		line = strings.TrimSpace(line)
		line = strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
		parts := strings.Split(line, "|")
		for k := range parts {
			parts[k] = strings.TrimSpace(parts[k])
		}
		return parts
	}
	header := cells(lines[i])
	row := func(values []string, kind string) *node {
		r := &node{Type: "tableRow"}
		for k := range header {
			p := &node{Type: "paragraph"}
			if k < len(values) && values[k] != "" {
				p.Content = mdInline(values[k], nil)
			}
			r.Content = append(r.Content, &node{Type: kind, Content: []*node{p}})
		}
		return r
	}
	table := &node{Type: "table", Content: []*node{row(header, "tableHeader")}}
	i += 2
	for i < len(lines) && strings.TrimSpace(lines[i]) != "" && strings.Contains(lines[i], "|") {
		table.Content = append(table.Content, row(cells(lines[i]), "tableCell"))
		i++
	}
	return table, i
}

// mdInline reads the marks of one line of text.
func mdInline(s string, marks []*mark) []*node {
	var out []*node
	var plain strings.Builder
	flush := func() {
		if plain.Len() > 0 {
			out = append(out, textNode(plain.String(), marks))
			plain.Reset()
		}
	}
	with := func(m *mark) []*mark { return append(append([]*mark{}, marks...), m) }
	for i := 0; i < len(s); {
		rest := s[i:]
		switch {
		case rest[0] == '\\' && len(rest) > 1 && strings.ContainsRune("\\`*_~[]()#>-!|", rune(rest[1])):
			plain.WriteByte(rest[1])
			i += 2
			continue
		case rest[0] == '`':
			ticks := len(rest) - len(strings.TrimLeft(rest, "`"))
			if end := strings.Index(rest[ticks:], rest[:ticks]); end > 0 {
				flush()
				out = append(out, textNode(strings.TrimSpace(rest[ticks:ticks+end]), with(&mark{Type: "code"})))
				i += 2*ticks + end
				continue
			}
		case strings.HasPrefix(rest, "**") || strings.HasPrefix(rest, "__"):
			if end := strings.Index(rest[2:], rest[:2]); end > 0 {
				// "***" closes the inner italic first, then the bold.
				for 2+end+2 < len(rest) && rest[2+end+2] == rest[0] {
					end++
				}
				flush()
				out = append(out, mdInline(rest[2:2+end], with(&mark{Type: "bold"}))...)
				i += 4 + end
				continue
			}
		case strings.HasPrefix(rest, "~~"):
			if end := strings.Index(rest[2:], "~~"); end > 0 {
				flush()
				out = append(out, mdInline(rest[2:2+end], with(&mark{Type: "strike"}))...)
				i += 4 + end
				continue
			}
		case (rest[0] == '*' || rest[0] == '_') && (i == 0 || !isWord(s[:i], true)):
			if end := closingEmphasis(rest[1:], rest[0]); end > 0 {
				flush()
				out = append(out, mdInline(rest[1:1+end], with(&mark{Type: "italic"}))...)
				i += 2 + end
				continue
			}
		case rest[0] == '[':
			if text, href, n, ok := mdLink(rest); ok {
				flush()
				out = append(out, mdInline(text, with(linkMark(href)))...)
				i += n
				continue
			}
		case rest[0] == '<':
			if end := strings.IndexByte(rest, '>'); end > 0 && mdURL.MatchString(rest[1:end]) {
				flush()
				out = append(out, textNode(rest[1:end], with(linkMark(rest[1:end]))))
				i += end + 1
				continue
			}
		case rest[0] == 'h' && (i == 0 || !isWord(s[:i], true)):
			if u := mdURL.FindString(rest); u != "" {
				flush()
				out = append(out, textNode(u, with(linkMark(u))))
				i += len(u)
				continue
			}
		}
		_, size := utf8.DecodeRuneInString(rest)
		plain.WriteString(rest[:size])
		i += size
	}
	flush()
	return out
}

// closingEmphasis finds the closing * or _ of an italic run that does not
// start or end with a space.
func closingEmphasis(s string, c byte) int {
	if s == "" || s[0] == ' ' {
		return -1
	}
	for k := 1; k < len(s); k++ {
		if s[k] != c || s[k-1] == ' ' || s[k-1] == '\\' {
			continue
		}
		if k+1 < len(s) && isWord(s[k+1:], false) {
			continue
		}
		if c == '*' && k+1 < len(s) && s[k+1] == '*' {
			k++
			continue
		}
		return k
	}
	return -1
}

func isWord(s string, last bool) bool {
	var r rune
	if last {
		r, _ = utf8.DecodeLastRuneInString(s)
	} else {
		r, _ = utf8.DecodeRuneInString(s)
	}
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// mdLink reads [text](href) and how many bytes it takes.
func mdLink(s string) (string, string, int, bool) {
	depth := 0
	for k := 0; k < len(s); k++ {
		switch s[k] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				if k+1 >= len(s) || s[k+1] != '(' {
					return "", "", 0, false
				}
				end := strings.IndexByte(s[k+2:], ')')
				if end < 0 {
					return "", "", 0, false
				}
				href := strings.TrimSpace(s[k+2 : k+2+end])
				if sp := strings.IndexByte(href, ' '); sp > 0 {
					href = href[:sp] // drops a "title"
				}
				if ok, _ := safeHref(jsonValue(href)); !ok {
					return "", "", 0, false
				}
				return s[1:k], href, k + 3 + end, true
			}
		}
	}
	return "", "", 0, false
}

func linkMark(href string) *mark {
	return &mark{Type: "link", Attrs: map[string]json.RawMessage{"href": jsonValue(href)}}
}

func textNode(s string, marks []*mark) *node {
	return &node{Type: "text", Text: &s, Marks: marks}
}

func jsonValue(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// ToMarkdown writes a stored document as Markdown. labels are the current
// names of its mentions, by id (RF-27 of spec 005).
func ToMarkdown(doc json.RawMessage, labels map[string]string) string {
	var root node
	if len(doc) == 0 || json.Unmarshal(doc, &root) != nil {
		return ""
	}
	w := mdWriter{labels: labels}
	return strings.TrimRight(w.blocks(root.Content, ""), "\n")
}

type mdWriter struct{ labels map[string]string }

func (w mdWriter) blocks(nodes []*node, prefix string) string {
	var b strings.Builder
	for k, n := range nodes {
		if k > 0 {
			b.WriteString(strings.TrimRight(prefix, " ") + "\n")
		}
		b.WriteString(w.block(n, prefix))
	}
	return b.String()
}

// block writes one block; prefix goes before every line but the first, which
// the caller has already started.
func (w mdWriter) block(n *node, prefix string) string {
	indentLines := func(s, first, rest string) string {
		lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
		for k := range lines {
			if k == 0 {
				lines[k] = first + lines[k]
			} else if lines[k] != "" {
				lines[k] = rest + lines[k]
			} else {
				lines[k] = strings.TrimRight(rest, " ")
			}
		}
		return strings.Join(lines, "\n") + "\n"
	}
	switch n.Type {
	case "paragraph":
		return indentLines(w.inline(n.Content), "", prefix)
	case "heading":
		level := 1
		_ = json.Unmarshal(n.Attrs["level"], &level)
		return strings.Repeat("#", max(level, 1)) + " " + strings.ReplaceAll(w.inline(n.Content), "\n", " ") + "\n"
	case "codeBlock":
		var lang string
		_ = json.Unmarshal(n.Attrs["language"], &lang)
		return indentLines("```"+lang+"\n"+plainText(n)+"\n```", "", prefix)
	case "blockquote", "callout":
		return indentLines(w.blocks(n.Content, ""), "> ", prefix+"> ")
	case "horizontalRule":
		return "---\n"
	case "bulletList", "orderedList", "taskList":
		start := 1
		_ = json.Unmarshal(n.Attrs["start"], &start)
		var b strings.Builder
		for k, item := range n.Content {
			marker := "- "
			switch {
			case n.Type == "orderedList":
				marker = strconv.Itoa(start+k) + ". "
			case item.Type == "taskItem":
				var checked bool
				_ = json.Unmarshal(item.Attrs["checked"], &checked)
				marker = "- [ ] "
				if checked {
					marker = "- [x] "
				}
			}
			if k > 0 {
				b.WriteString(prefix)
			}
			pad := prefix + strings.Repeat(" ", len(marker))
			b.WriteString(marker + w.tight(item.Content, pad))
		}
		return b.String()
	case "details":
		var b strings.Builder
		for k, c := range n.Content {
			if k > 0 {
				b.WriteString(prefix)
			}
			if c.Type == "detailsSummary" {
				b.WriteString("**" + w.inline(c.Content) + "**\n")
			} else {
				b.WriteString(w.blocks(c.Content, prefix))
			}
		}
		return b.String()
	case "table":
		var b strings.Builder
		for k, row := range n.Content {
			if k > 0 {
				b.WriteString(prefix)
			}
			var cells []string
			for _, c := range row.Content {
				cells = append(cells, strings.ReplaceAll(strings.TrimSpace(w.blocks(c.Content, "")), "\n", " "))
			}
			b.WriteString("| " + strings.Join(cells, " | ") + " |\n")
			if k == 0 {
				b.WriteString(prefix + "|" + strings.Repeat(" --- |", len(cells)) + "\n")
			}
		}
		return b.String()
	case "image":
		var alt string
		_ = json.Unmarshal(n.Attrs["alt"], &alt)
		return "[imagen" + optional(alt) + "]\n"
	case "fileAttachment":
		return "[archivo adjunto]\n"
	}
	return w.blocks(n.Content, prefix)
}

// tight writes the blocks of a list item with no blank line between its
// paragraph and a nested list.
func (w mdWriter) tight(nodes []*node, pad string) string {
	var b strings.Builder
	for k, c := range nodes {
		if k > 0 {
			b.WriteString(pad)
		}
		b.WriteString(w.block(c, pad))
	}
	if b.Len() == 0 {
		return "\n"
	}
	return b.String()
}

func optional(s string) string {
	if s == "" {
		return ""
	}
	return ": " + s
}

func plainText(n *node) string {
	var b strings.Builder
	for _, c := range n.Content {
		if c.Text != nil {
			b.WriteString(*c.Text)
		} else if c.Type == "hardBreak" {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (w mdWriter) inline(nodes []*node) string {
	var b strings.Builder
	for _, n := range nodes {
		switch n.Type {
		case "hardBreak":
			b.WriteString("\n")
		case "mention":
			var id string
			_ = json.Unmarshal(n.Attrs["id"], &id)
			label := w.labels[id]
			if label == "" {
				label = "mención"
			}
			b.WriteString("@" + label)
		case "text":
			if n.Text == nil {
				continue
			}
			s := *n.Text
			var href string
			for _, m := range n.Marks {
				switch m.Type {
				case "code":
					s = "`" + s + "`"
				case "bold":
					s = "**" + s + "**"
				case "italic":
					s = "*" + s + "*"
				case "strike":
					s = "~~" + s + "~~"
				case "link":
					_ = json.Unmarshal(m.Attrs["href"], &href)
				}
			}
			if href != "" {
				s = "[" + s + "](" + href + ")"
			}
			b.WriteString(s)
		}
	}
	return b.String()
}
