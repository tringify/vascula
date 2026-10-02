package docsite

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode"
)

// The documentation uses a small, strict subset of Markdown: headings,
// paragraphs, lists, tables, fenced code blocks, block quotes and inline
// code, emphasis and links. Anything else is written as text. Keeping the
// renderer here, without dependencies, lets the site build offline from the
// source distribution.

// heading is an h2/h3 for the page's table of contents.
type heading struct {
	Level int
	ID    string
	Text  string
}

// page is one rendered document.
type rendered struct {
	HTML     string
	Headings []heading
	Links    []string // internal link targets, for checking
	Text     string   // plain text, for search
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// slugify makes a heading id: "Comparing values" → "comparing-values".
func slugify(s string) string {
	s = strings.ToLower(stripInline(s))
	s = nonSlug.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// stripInline removes inline Markdown markup, leaving the text.
func stripInline(s string) string {
	s = linkPattern.ReplaceAllString(s, "$1")
	return strings.NewReplacer("`", "", "**", "", "\\|", "|").Replace(s)
}

type mdBlock struct {
	kind  string // code
	lang  string
	flags []string
	body  string
}

func renderMarkdown(src string) (rendered, error) {
	var r rendered
	var out strings.Builder
	var text strings.Builder
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	ids := map[string]int{}
	var pendingExample []mdBlock

	flushExample := func() {
		if len(pendingExample) == 0 {
			return
		}
		out.WriteString(renderExample(pendingExample))
		pendingExample = nil
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			continue

		case strings.HasPrefix(trimmed, "```"):
			info := strings.Fields(strings.TrimPrefix(trimmed, "```"))
			b := mdBlock{kind: "code"}
			if len(info) > 0 {
				b.lang, b.flags = info[0], info[1:]
			}
			var body []string
			closed := false
			for i++; i < len(lines); i++ {
				if strings.TrimSpace(lines[i]) == "```" {
					closed = true
					break
				}
				body = append(body, lines[i])
			}
			if !closed {
				return r, fmt.Errorf("unclosed code block")
			}
			b.body = strings.Join(body, "\n")
			text.WriteString(b.body + "\n")
			// A vascula block followed by data and output blocks renders as
			// one example; other blocks render on their own.
			switch {
			case b.lang == "vascula" && !has(b.flags, "fragment"):
				flushExample()
				pendingExample = []mdBlock{b}
			case len(pendingExample) > 0 && ((b.lang == "json" && has(b.flags, "data")) || b.lang == "output" || b.lang == "error"):
				pendingExample = append(pendingExample, b)
			case b.lang == "go" && has(b.flags, "run"):
				flushExample()
				pendingExample = []mdBlock{b}
			default:
				flushExample()
				out.WriteString(renderCode(b.lang, b.body, ""))
			}
			continue
		}
		flushExample()

		switch {
		case strings.HasPrefix(trimmed, "#"):
			level := len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
			if level > 4 || len(trimmed) <= level || trimmed[level] != ' ' {
				return r, fmt.Errorf("bad heading %q", trimmed)
			}
			content := strings.TrimSpace(trimmed[level:])
			id := slugify(content)
			if n := ids[id]; n > 0 {
				id = fmt.Sprintf("%s-%d", id, n+1)
			}
			ids[slugify(content)]++
			if level == 2 || level == 3 {
				r.Headings = append(r.Headings, heading{Level: level, ID: id, Text: stripInline(content)})
			}
			text.WriteString(stripInline(content) + "\n")
			if level == 1 {
				fmt.Fprintf(&out, "<h1>%s</h1>\n", inline(content, &r))
			} else {
				fmt.Fprintf(&out, `<h%d id="%s"><a class="anchor" href="#%s" aria-label="Link to this section">#</a>%s</h%d>`+"\n", level, id, id, inline(content, &r), level)
			}

		case strings.HasPrefix(trimmed, "|"):
			var rows []string
			for ; i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "|"); i++ {
				rows = append(rows, strings.TrimSpace(lines[i]))
			}
			i--
			table, err := renderTable(rows, &r, &text)
			if err != nil {
				return r, err
			}
			out.WriteString(table)

		case strings.HasPrefix(trimmed, "> "):
			var quote []string
			for ; i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), ">"); i++ {
				quote = append(quote, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[i]), ">")))
			}
			i--
			body := strings.Join(quote, " ")
			text.WriteString(stripInline(body) + "\n")
			fmt.Fprintf(&out, "<blockquote><p>%s</p></blockquote>\n", inline(body, &r))

		case isListItem(trimmed):
			ordered := !strings.HasPrefix(trimmed, "- ")
			var items []string
			for ; i < len(lines); i++ {
				l := lines[i]
				t := strings.TrimSpace(l)
				if t == "" {
					// A blank line ends the list unless an item follows.
					if i+1 < len(lines) && isListItem(strings.TrimSpace(lines[i+1])) {
						continue
					}
					break
				}
				if isListItem(t) && !strings.HasPrefix(l, "  ") {
					items = append(items, listItemText(t))
					continue
				}
				if strings.HasPrefix(l, "  ") && len(items) > 0 {
					items[len(items)-1] += " " + t
					continue
				}
				break
			}
			i--
			tag := "ul"
			if ordered {
				tag = "ol"
			}
			fmt.Fprintf(&out, "<%s>\n", tag)
			for _, item := range items {
				text.WriteString(stripInline(item) + "\n")
				fmt.Fprintf(&out, "<li>%s</li>\n", inline(item, &r))
			}
			fmt.Fprintf(&out, "</%s>\n", tag)

		default:
			var para []string
			for ; i < len(lines); i++ {
				t := strings.TrimSpace(lines[i])
				if t == "" || strings.HasPrefix(t, "```") || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "|") || strings.HasPrefix(t, "> ") {
					break
				}
				para = append(para, t)
			}
			i--
			body := strings.Join(para, " ")
			text.WriteString(stripInline(body) + "\n")
			fmt.Fprintf(&out, "<p>%s</p>\n", inline(body, &r))
		}
	}
	flushExample()
	r.HTML = out.String()
	r.Text = text.String()
	return r, nil
}

func has(flags []string, flag string) bool {
	for _, f := range flags {
		if f == flag {
			return true
		}
	}
	return false
}

var orderedItem = regexp.MustCompile(`^\d+\. `)

func isListItem(t string) bool {
	return strings.HasPrefix(t, "- ") || orderedItem.MatchString(t)
}

func listItemText(t string) string {
	if strings.HasPrefix(t, "- ") {
		return strings.TrimPrefix(t, "- ")
	}
	return orderedItem.ReplaceAllString(t, "")
}

// splitRow splits a table row on unescaped pipes outside code spans.
func splitRow(row string) []string {
	row = strings.TrimSpace(row)
	row = strings.TrimPrefix(row, "|")
	row = strings.TrimSuffix(row, "|")
	var cells []string
	var cur strings.Builder
	inCode := false
	for i := 0; i < len(row); i++ {
		c := row[i]
		switch {
		case c == '\\' && i+1 < len(row) && row[i+1] == '|':
			cur.WriteString(`\|`)
			i++
		case c == '`':
			inCode = !inCode
			cur.WriteByte(c)
		case c == '|' && !inCode:
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	return append(cells, strings.TrimSpace(cur.String()))
}

func renderTable(rows []string, r *rendered, text *strings.Builder) (string, error) {
	if len(rows) < 2 {
		return "", fmt.Errorf("table needs a header and a separator row")
	}
	header := splitRow(rows[0])
	sep := splitRow(rows[1])
	if len(sep) != len(header) {
		return "", fmt.Errorf("table separator has %d columns, header %d", len(sep), len(header))
	}
	align := make([]string, len(sep))
	for i, s := range sep {
		if strings.HasSuffix(s, ":") && !strings.HasPrefix(s, ":") {
			align[i] = ` class="num"`
		}
	}
	var b strings.Builder
	b.WriteString("<div class=\"table\"><table>\n<thead><tr>")
	for i, h := range header {
		text.WriteString(stripInline(h) + " ")
		fmt.Fprintf(&b, "<th%s>%s</th>", align[i], inline(h, r))
	}
	b.WriteString("</tr></thead>\n<tbody>\n")
	for _, row := range rows[2:] {
		cells := splitRow(row)
		if len(cells) != len(header) {
			return "", fmt.Errorf("table row %q has %d cells, want %d", row, len(cells), len(header))
		}
		b.WriteString("<tr>")
		for i, c := range cells {
			text.WriteString(stripInline(c) + " ")
			fmt.Fprintf(&b, "<td%s>%s</td>", align[i], inline(c, r))
		}
		b.WriteString("</tr>\n")
		text.WriteString("\n")
	}
	b.WriteString("</tbody></table></div>\n")
	return b.String(), nil
}

var linkPattern = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)

// inline renders code spans, links, bold and italic. Code spans are taken
// out first, so nothing inside them is interpreted.
func inline(s string, r *rendered) string {
	var out strings.Builder
	for len(s) > 0 {
		i := strings.IndexByte(s, '`')
		if i < 0 {
			out.WriteString(inlineText(s, r))
			break
		}
		j := strings.IndexByte(s[i+1:], '`')
		if j < 0 {
			out.WriteString(inlineText(s, r))
			break
		}
		out.WriteString(inlineText(s[:i], r))
		code := strings.ReplaceAll(s[i+1:i+1+j], `\|`, "|")
		out.WriteString("<code>" + html.EscapeString(code) + "</code>")
		s = s[i+2+j:]
	}
	return out.String()
}

var boldPattern = regexp.MustCompile(`\*\*([^*]+)\*\*`)
var italicPattern = regexp.MustCompile(`(^|[^\w*])\*([^*\s][^*]*)\*`)

func inlineText(s string, r *rendered) string {
	// Links first, on the raw text, so their targets are not escaped twice.
	var out strings.Builder
	last := 0
	for _, m := range linkPattern.FindAllStringSubmatchIndex(s, -1) {
		out.WriteString(emphasis(html.EscapeString(unescapePipes(s[last:m[0]]))))
		label := s[m[2]:m[3]]
		target := s[m[4]:m[5]]
		attrs := ""
		if strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "http://") {
			attrs = ` rel="noopener"`
		} else {
			r.Links = append(r.Links, target)
		}
		fmt.Fprintf(&out, `<a href="%s"%s>%s</a>`, html.EscapeString(target), attrs, inline(label, r))
		last = m[1]
	}
	out.WriteString(emphasis(html.EscapeString(unescapePipes(s[last:]))))
	return out.String()
}

func unescapePipes(s string) string { return strings.ReplaceAll(s, `\|`, "|") }

func emphasis(s string) string {
	s = boldPattern.ReplaceAllString(s, "<strong>$1</strong>")
	return italicPattern.ReplaceAllStringFunc(s, func(m string) string {
		sub := italicPattern.FindStringSubmatch(m)
		return sub[1] + "<em>" + sub[2] + "</em>"
	})
}

// ---- code ----

var (
	goKeywords = regexp.MustCompile(`\b(package|import|func|return|if|else|for|range|var|const|type|struct|map|interface|nil|true|false|err|go|defer|switch|case|default|break|continue)\b`)
)

// renderCode renders a code block with light syntax highlighting.
func renderCode(lang, body, label string) string {
	var hl string
	switch lang {
	case "vascula":
		hl = highlightVascula(body)
	case "go":
		hl = highlightGo(body)
	case "json":
		hl = highlightJSON(body)
	default:
		hl = html.EscapeString(body)
	}
	cls := "lang-" + lang
	if lang == "" {
		cls = "lang-text"
	}
	labelHTML := ""
	if label != "" {
		labelHTML = `<span class="code-label">` + html.EscapeString(label) + `</span>`
	}
	return fmt.Sprintf(`<div class="code %s">%s<button class="copy" type="button" aria-label="Copy">Copy</button><pre><code>%s</code></pre></div>`+"\n", cls, labelHTML, hl)
}

func renderExample(blocks []mdBlock) string {
	var b strings.Builder
	b.WriteString(`<div class="example">`)
	for _, blk := range blocks {
		label := map[string]string{"vascula": "Template", "json": "Data", "output": "Output", "error": "Error", "go": "Go"}[blk.lang]
		if blk.lang == "output" && len(blocks) > 0 && blocks[0].lang == "go" {
			label = "Prints"
		}
		lang := blk.lang
		if lang == "output" || lang == "error" {
			lang = "text"
		}
		b.WriteString(strings.Replace(renderCode(lang, blk.body, label), `class="code `, `class="code part-`+blk.lang+` `, 1))
	}
	b.WriteString("</div>\n")
	return b.String()
}

func span(class, s string) string {
	return `<span class="` + class + `">` + s + `</span>`
}

// highlightVascula marks delimiters, keywords, strings, filters and comments.
func highlightVascula(src string) string {
	var out strings.Builder
	for len(src) > 0 {
		i := strings.Index(src, "{")
		for i >= 0 && i+1 < len(src) && src[i+1] != '{' && src[i+1] != '%' && src[i+1] != '#' {
			next := strings.Index(src[i+1:], "{")
			if next < 0 {
				i = -1
				break
			}
			i += 1 + next
		}
		if i < 0 || i+1 >= len(src) {
			out.WriteString(span("t-text", html.EscapeString(src)))
			break
		}
		out.WriteString(span("t-text", html.EscapeString(src[:i])))
		open := src[i : i+2]
		close := map[string]string{"{{": "}}", "{%": "%}", "{#": "#}"}[open]
		j := strings.Index(src[i+2:], close)
		if j < 0 {
			out.WriteString(html.EscapeString(src[i:]))
			break
		}
		inner := src[i+2 : i+2+j]
		if open == "{#" {
			out.WriteString(span("t-comment", html.EscapeString(open+inner+close)))
		} else {
			out.WriteString(span("t-delim", html.EscapeString(open)) + highlightExpr(inner, open == "{%") + span("t-delim", html.EscapeString(close)))
		}
		src = src[i+2+j+2:]
	}
	return out.String()
}

var exprToken = regexp.MustCompile(`"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'|-?\d+(?:\.\d+)?|[A-Za-z_][A-Za-z0-9_]*|\|\s*[A-Za-z_][A-Za-z0-9_]*|\s+|.`)

var tagKeywords = map[string]bool{
	"if": true, "elsif": true, "else": true, "endif": true, "unless": true, "endunless": true,
	"case": true, "when": true, "endcase": true, "for": true, "endfor": true, "in": true,
	"break": true, "continue": true, "cycle": true, "assign": true, "capture": true,
	"endcapture": true, "render": true, "form": true, "endform": true, "comment": true,
	"endcomment": true, "limit": true, "offset": true, "reversed": true,
}
var exprKeywords = map[string]bool{"and": true, "or": true, "contains": true, "true": true, "false": true, "nil": true, "null": true, "empty": true, "blank": true}

func highlightExpr(src string, isTag bool) string {
	var out strings.Builder
	first := true
	for _, tok := range exprToken.FindAllString(src, -1) {
		switch {
		case strings.TrimSpace(tok) == "":
			out.WriteString(tok)
			continue
		case tok[0] == '"' || tok[0] == '\'':
			out.WriteString(span("t-string", html.EscapeString(tok)))
		case tok[0] == '|':
			name := strings.TrimLeft(tok[1:], " \t")
			out.WriteString(span("t-op", "|") + tok[1:len(tok)-len(name)] + span("t-filter", html.EscapeString(name)))
		case unicode.IsDigit(rune(tok[0])) || (tok[0] == '-' && len(tok) > 1):
			out.WriteString(span("t-number", html.EscapeString(tok)))
		case isTag && tagKeywords[tok] && (first || tok == "in" || tok == "limit" || tok == "offset" || tok == "reversed"):
			out.WriteString(span("t-keyword", html.EscapeString(tok)))
		case exprKeywords[tok]:
			out.WriteString(span("t-keyword", html.EscapeString(tok)))
		case unicode.IsLetter(rune(tok[0])) || tok[0] == '_':
			out.WriteString(span("t-name", html.EscapeString(tok)))
		default:
			out.WriteString(span("t-op", html.EscapeString(tok)))
		}
		first = false
	}
	return out.String()
}

var goToken = regexp.MustCompile("(?s)//[^\n]*|`[^`]*`|\"(?:[^\"\\\\]|\\\\.)*\"|'(?:[^'\\\\]|\\\\.)*'|[A-Za-z_][A-Za-z0-9_]*|\\d+(?:\\.\\d+)?|\\s+|.")

func highlightGo(src string) string {
	var out strings.Builder
	for _, tok := range goToken.FindAllString(src, -1) {
		switch {
		case strings.HasPrefix(tok, "//"):
			out.WriteString(span("t-comment", html.EscapeString(tok)))
		case tok[0] == '"' || tok[0] == '`' || tok[0] == '\'':
			out.WriteString(span("t-string", html.EscapeString(tok)))
		case goKeywords.MatchString(tok) && goKeywords.FindString(tok) == tok:
			out.WriteString(span("t-keyword", html.EscapeString(tok)))
		case unicode.IsDigit(rune(tok[0])):
			out.WriteString(span("t-number", html.EscapeString(tok)))
		default:
			out.WriteString(html.EscapeString(tok))
		}
	}
	return out.String()
}

var jsonToken = regexp.MustCompile(`"(?:[^"\\]|\\.)*"(\s*:)?|-?\d+(?:\.\d+)?|true|false|null|\s+|.`)

func highlightJSON(src string) string {
	var out strings.Builder
	for _, m := range jsonToken.FindAllStringSubmatch(src, -1) {
		tok := m[0]
		switch {
		case tok[0] == '"' && m[1] != "":
			key := strings.TrimSuffix(strings.TrimRight(tok, " \t:"), "")
			out.WriteString(span("t-name", html.EscapeString(key)) + html.EscapeString(tok[len(key):]))
		case tok[0] == '"':
			out.WriteString(span("t-string", html.EscapeString(tok)))
		case tok == "true" || tok == "false" || tok == "null":
			out.WriteString(span("t-keyword", tok))
		case unicode.IsDigit(rune(tok[0])) || (tok[0] == '-' && len(tok) > 1):
			out.WriteString(span("t-number", tok))
		default:
			out.WriteString(html.EscapeString(tok))
		}
	}
	return out.String()
}
