package vascula

import (
	"bytes"
	"fmt"
	"strings"
)

// Output context.
//
// At compile time the template's literal markup is scanned, and every output
// tag learns where it prints:
//
//   - Inside a tag (an attribute value), the json filter's output is
//     HTML-escaped on write: its quotes would end an attribute value. There
//     `{{ x | json }}` is as safe as `{{ x | json | escape }}`, which renders
//     exactly as before. In <script> json stays raw; its encoder escapes <, >
//     and &.
//   - In an unquoted attribute value, every character that would end the value
//     or start another attribute is escaped as well (see unquotedEscaper).
//   - In the value of a URL-bearing attribute, the attribute's value is
//     checked as a whole after rendering (see url_context.go).
//   - Where an attribute would go (`<img src="…" {{ image_attributes }}>`),
//     attribute markup the host vouched for (HTMLAttr) prints as written; any other
//     value must be a single attribute name that is not an event handler,
//     style or URL-bearing attribute (`hidden`), and otherwise prints nothing.
//
// Nothing can print an element name or part of an attribute name: `<h{{ n }}>`,
// `<a {{ name }}="…">`, `<a data-{{ key }}="…">` and two printed values with
// no space between them in a tag fail to compile. {% render %} cannot print
// inside a tag except in a quoted attribute value.
//
// As in Go's html/template, the scan follows control flow: every branch of an
// if, unless or case starts where the tag stands and all must end in the same
// context (an if without else ends where it started), a loop body must end
// where it started, and {% break %} and {% continue %} must stand where their
// loop started. A template and a capture body must not end inside a tag.
// A name is never split by control flow: when a branch ends inside an element
// or attribute name (`<input {% if c %}checked{% endif %}>`), what follows the
// branch must end the name (a space, "/", "=" or ">"); text or output that
// would continue it (`<a {% if c %}h{% endif %}ref="…">`) fails to compile, as
// do a loop body or {% break %}/{% continue %} ending inside a name and a loop
// starting inside one.

type htmlState int

const (
	htmlData htmlState = iota
	htmlTagName
	htmlInTag
	htmlAttrName
	htmlAfterAttrName
	htmlBeforeValue
	htmlValueDouble
	htmlValueSingle
	htmlValueUnquoted
	htmlRawText
	htmlComment
)

// printContext is how an output tag prints.
type printContext struct {
	// inTag: inside an HTML tag, where json text is HTML-escaped.
	inTag bool
	// unquoted: in an unquoted attribute value.
	unquoted bool
	// cssURL: the output starts the argument of url() in a style attribute.
	cssURL bool
	// afterLT: the output follows a bare "<" in text, where a letter, "/", "!"
	// or "?" would start a tag; such a first character prints as a reference.
	afterLT bool
	// attrName: the output stands where an attribute goes.
	attrName bool
}

// htmlCtx is the scanner's position in the markup. It is a value, so a branch
// can start from a copy.
type htmlCtx struct {
	state htmlState
	// tag is the name of the tag being read; closing marks </name.
	tag     string
	closing bool
	// raw is the element whose text ends only at its end tag (script, style).
	raw string
	// tail holds the last characters read (lowercased) in raw text, a comment
	// or a style value, enough to recognise an end marker or "url(" across
	// text nodes, without allocating per character.
	tail [16]byte
	// opener is what followed "<!": "--" starts a comment that only "-->"
	// ends; anything else (a doctype) ends at ">".
	opener string
	// attr is the name of the attribute being read, value the kind of its
	// value, and open the URL-bearing value being read, if any.
	// valuePos is the source offset where the value starts.
	attr     string
	value    attrKind
	open     *urlAttr
	valuePos int
	// printedAttr: something printed where an attribute goes and no space has
	// followed it yet.
	printedAttr bool
	// ambiguousName: branches ended in attribute names of different kinds;
	// only a value for that name would be ambiguous.
	ambiguousName bool
	// nameOpen: a path through the control flow just closed ended inside an
	// element or attribute name, which the next character would continue on
	// that path only.
	nameOpen bool
}

func (c *htmlCtx) inTag() bool {
	switch c.state {
	case htmlTagName, htmlInTag, htmlAttrName, htmlAfterAttrName, htmlBeforeValue,
		htmlValueDouble, htmlValueSingle, htmlValueUnquoted:
		return true
	}
	return false
}

// inName reports whether the next character could continue an element or
// attribute name.
func (c *htmlCtx) inName() bool {
	return c.state == htmlTagName || c.state == htmlAttrName || c.nameOpen
}

func (c *htmlCtx) inValue() bool {
	return c.state == htmlValueDouble || c.state == htmlValueSingle || c.state == htmlValueUnquoted
}

// htmlMarker scans a template's nodes and returns them annotated, with
// attribute markers inserted.
type htmlMarker struct {
	ctx htmlCtx
	// loops holds the context at the start of each enclosing loop body.
	loops []htmlCtx
	err   error
	// out collects the nodes of the list being scanned.
	out []node
	// emit: this pass rewrites the nodes. dynamic holds the source offsets of
	// URL attribute values something prints into, found by the first pass.
	emit    bool
	dynamic map[int]bool
	// tagFrom and attrFrom are where the tag or attribute name being read
	// starts in the current text, or -1; names are lowercased into the
	// context when they end, once per name rather than per character.
	tagFrom, attrFrom int
}

func (m *htmlMarker) fail(pos int, format string, args ...interface{}) {
	if m.err == nil {
		m.err = atOffset(pos, format, args...)
	}
}

// markOutputContexts annotates every output with its context. It fails when an
// output cannot be printed safely where it stands or control flow leaves the
// markup ambiguous.
func markOutputContexts(nodes []node, src string) ([]node, error) {
	// The first pass finds the URL attribute values something prints into;
	// the second annotates the nodes and brackets only those values, so
	// attributes written entirely in the template cost nothing.
	m := &htmlMarker{dynamic: map[int]bool{}}
	m.mark(nodes)
	if m.err == nil && m.ctx.inTag() {
		m.fail(len(src), "the template ends inside the <%s> tag; close the tag in this template", m.ctx.tag)
	}
	if m.err != nil {
		return nil, m.err
	}
	if len(m.dynamic) == 0 {
		m = &htmlMarker{emit: true}
	} else {
		m = &htmlMarker{emit: true, dynamic: m.dynamic}
	}
	return m.mark(nodes), m.err
}

func (m *htmlMarker) mark(nodes []node) []node {
	saved := m.out
	if m.emit {
		m.out = make([]node, 0, len(nodes))
	}
	for _, n := range nodes {
		switch x := n.(type) {
		case textNode:
			m.text(n, x.text, x.pos)
			continue
		case outputNode:
			x.inHTMLText = m.ctx.state == htmlData
			x.at = m.printAt(x.pos, "an output tag", true)
			n = x
		case cycleNode:
			x.at = m.printAt(x.pos, "{% cycle %}", true)
			n = x
		case renderNode:
			if s := m.ctx.state; s == htmlInTag || s == htmlAfterAttrName {
				m.fail(x.pos, "{%% render %%} cannot print attributes in <%s>", m.ctx.tag)
			}
			m.printAt(x.pos, "{% render %}", false)
		case hostTagNode:
			m.printAt(x.pos, "a host tag", false)
		case formNode:
			if m.ctx.inTag() {
				m.fail(x.pos, "{%% form %%} cannot stand inside an HTML tag")
			}
			x.body = m.mark(x.body)
			n = x
		case ifNode:
			start := m.ctx
			ends := make([]htmlCtx, 0, len(x.branches)+1)
			for b := range x.branches {
				m.ctx = start
				x.branches[b].body = m.mark(x.branches[b].body)
				ends = append(ends, m.ctx)
			}
			m.ctx = start
			x.elseBody = m.mark(x.elseBody)
			ends = append(ends, m.ctx)
			m.ctx = m.join(ends, x.pos, "{% if %} branches")
			n = x
		case caseNode:
			start := m.ctx
			ends := make([]htmlCtx, 0, len(x.whens)+1)
			for w := range x.whens {
				m.ctx = start
				x.whens[w].body = m.mark(x.whens[w].body)
				ends = append(ends, m.ctx)
			}
			m.ctx = start
			x.elseBody = m.mark(x.elseBody)
			ends = append(ends, m.ctx)
			m.ctx = m.join(ends, x.pos, "{% case %} branches")
			n = x
		case forNode:
			start := m.ctx
			if start.inName() {
				m.fail(x.pos, "{%% for %%} cannot start inside a name in <%s>; end the name with a space before the loop", start.tag)
			}
			m.loops = append(m.loops, start)
			x.body = m.mark(x.body)
			m.loops = m.loops[:len(m.loops)-1]
			if m.ctx.inName() {
				m.fail(x.pos, "the {%% for %%} body ends inside a name in <%s>, which the next pass would continue; end it with a space", m.ctx.tag)
			}
			m.join([]htmlCtx{start, m.ctx}, x.pos, "the {% for %} body")
			m.ctx = start
			x.elseBody = m.mark(x.elseBody)
			m.ctx = m.join([]htmlCtx{start, m.ctx}, x.pos, "{% for %} and its {% else %}")
			n = x
		case captureNode:
			// A capture prints nothing where it stands; its body is markup of
			// its own, read from the start.
			saved, loops := m.ctx, m.loops
			m.ctx = htmlCtx{}
			if len(loops) > 0 {
				m.loops = []htmlCtx{{}}
			}
			x.body = m.mark(x.body)
			if m.err == nil && m.ctx.inTag() {
				m.fail(x.pos, "the {%% capture %%} body ends inside the <%s> tag", m.ctx.tag)
			}
			m.ctx, m.loops = saved, loops
			n = x
		case breakNode:
			m.checkLoopExit(x.pos, "{% break %}")
		case continueNode:
			m.checkLoopExit(x.pos, "{% continue %}")
		}
		if m.emit {
			m.out = append(m.out, n)
		}
	}
	out := m.out
	m.out = saved
	if !m.emit {
		return nodes
	}
	return out
}

func (m *htmlMarker) checkLoopExit(pos int, what string) {
	if len(m.loops) == 0 {
		return
	}
	if m.ctx.inName() {
		m.fail(pos, "%s stands inside a name in <%s>; end the name with a space first", what, m.ctx.tag)
		return
	}
	if _, ok := joinCtx(m.ctx, m.loops[len(m.loops)-1]); !ok {
		m.fail(pos, "%s stands %s, but its loop starts %s", what, describeCtx(m.ctx), describeCtx(m.loops[len(m.loops)-1]))
	}
}

// printAt returns how something that prints stands at the current position,
// failing where it would choose markup rather than a value. quotable reports
// whether it may print an unquoted attribute value.
func (m *htmlMarker) printAt(pos int, what string, quotable bool) printContext {
	c := &m.ctx
	if c.nameOpen {
		c.nameOpen = false
		m.fail(pos, "%s follows control flow that may end inside a name in <%s>; end the name with a space inside the branch", what, c.tag)
	}
	switch {
	case c.state == htmlTagName && c.tag == "" && !c.closing:
		// "<" then something printed: text, whatever is printed.
		c.state = htmlData
		return printContext{afterLT: true}
	case c.state == htmlTagName:
		m.fail(pos, "%s cannot print an element name; write the element name in the template", what)
		return printContext{}
	}
	switch c.state {
	case htmlAttrName:
		m.fail(pos, "%s cannot print part of an attribute name in <%s>; write the attribute name in the template and print only its value", what, c.tag)
	case htmlInTag, htmlAfterAttrName:
		if c.printedAttr {
			m.fail(pos, "%s follows another printed value in <%s> with no space between them", what, c.tag)
		}
		c.state = htmlInTag
		c.printedAttr = true
		return printContext{inTag: true, attrName: true}
	case htmlBeforeValue:
		if !quotable {
			m.fail(pos, "%s cannot print the unquoted %s attribute value; quote the value", what, c.attr)
		}
		m.startValue(htmlValueUnquoted, pos)
	case htmlValueUnquoted:
		if !quotable {
			m.fail(pos, "%s cannot print the unquoted %s attribute value; quote the value", what, c.attr)
		}
	}
	if c.inValue() && c.value != attrPlain {
		if m.dynamic != nil && !m.emit {
			m.dynamic[c.valuePos] = true
		}
		if c.open != nil {
			c.open.dynamic = true
		}
	}
	return printContext{
		inTag:    c.inTag(),
		unquoted: c.state == htmlValueUnquoted,
		cssURL:   c.inValue() && c.value == attrStyle && opensCSSURL(string(c.tail[:])),
	}
}

// startValue starts the value of the attribute just named; a URL-bearing
// value is bracketed by markers.
func (m *htmlMarker) startValue(state htmlState, at int) {
	c := &m.ctx
	c.state = state
	c.value = classifyAttr(c.tag, c.attr)
	c.valuePos = at
	c.tail = [16]byte{}
	c.open = nil
	if m.marks(at) {
		c.open = &urlAttr{kind: c.value, unquoted: state == htmlValueUnquoted}
		m.out = append(m.out, attrOpenNode{a: c.open})
	}
}

// marks reports whether this pass brackets the value starting at source
// offset at.
func (m *htmlMarker) marks(at int) bool { return m.emit && m.dynamic[at] }

func (m *htmlMarker) endValue() {
	c := &m.ctx
	if c.open != nil {
		m.out = append(m.out, attrCloseNode{a: c.open})
		c.open = nil
	}
}

// text scans literal markup n, splitting it where an attribute marker goes.
func (m *htmlMarker) text(n node, t string, pos int) {
	c := &m.ctx
	seg := 0
	// flush emits the text before i; markers then follow it. Text read whole
	// is the node itself.
	flush := func(i int) {
		switch {
		case !m.emit:
		case i == len(t) && seg == 0:
			m.out = append(m.out, n)
		case i > seg:
			m.out = append(m.out, textNode{text: t[seg:i], pos: pos + seg})
		}
		seg = i
	}
	m.tagFrom, m.attrFrom = -1, -1
	for i := 0; i < len(t); i++ {
		ch := t[i]
		if c.nameOpen {
			c.nameOpen = false
			if !isHTMLSpace(ch) && ch != '/' && ch != '=' && ch != '>' {
				m.fail(pos+i, "%q in <%s> would continue a name that control flow may end inside; write the whole element or attribute name in one branch", ch, c.tag)
			}
		}
		switch c.state {
		case htmlData:
			if ch == '<' {
				c.state = htmlTagName
				c.tag = ""
				c.closing = false
			}
		case htmlTagName:
			named := c.tag != "" || m.tagFrom >= 0
			switch {
			case ch == '/' && !named && !c.closing:
				c.closing = true
			case ch == '!' && !named && !c.closing:
				c.state = htmlComment
				c.tail = [16]byte{}
				c.opener = ""
			case isASCIILetter(ch) || (named && (isASCIIDigit(ch) || ch == '-')):
				if m.tagFrom < 0 {
					m.tagFrom = i
				}
			case !named:
				// "<" not followed by a letter, "/" or "!" ("a < b") is text.
				c.state = htmlData
			case ch == '>':
				m.endName(t, i)
				m.closeTag()
			default:
				m.endName(t, i)
				c.state = htmlInTag
			}
		case htmlInTag:
			if c.printedAttr && !isHTMLSpace(ch) && ch != '>' && ch != '/' {
				m.fail(pos+i, "a printed value in <%s> is followed by %q; write the attribute name in the template and print only its value", c.tag, ch)
			}
			c.printedAttr = false
			switch {
			case ch == '>':
				m.closeTag()
			case isHTMLSpace(ch) || ch == '/':
			default:
				m.startAttr(i)
			}
		case htmlAttrName:
			switch {
			case ch == '>':
				m.endName(t, i)
				m.closeTag()
			case ch == '=':
				m.endName(t, i)
				m.checkNamed(pos + i)
				c.state = htmlBeforeValue
			case isHTMLSpace(ch):
				m.endName(t, i)
				c.state = htmlAfterAttrName
			case m.attrFrom < 0:
				m.attrFrom = i
			}
		case htmlAfterAttrName:
			switch {
			case ch == '>':
				m.closeTag()
			case ch == '=':
				m.checkNamed(pos + i)
				c.state = htmlBeforeValue
			case isHTMLSpace(ch) || ch == '/':
			default:
				m.startAttr(i)
			}
		case htmlBeforeValue:
			switch {
			case ch == '"' || ch == '\'':
				if m.marks(pos + i + 1) {
					flush(i + 1)
				}
				if ch == '"' {
					m.startValue(htmlValueDouble, pos+i+1)
				} else {
					m.startValue(htmlValueSingle, pos+i+1)
				}
			case ch == '>':
				m.closeTag()
			case isHTMLSpace(ch):
			default:
				if m.marks(pos + i) {
					flush(i)
				}
				m.startValue(htmlValueUnquoted, pos+i)
				m.valueText(ch)
			}
		case htmlValueDouble, htmlValueSingle:
			if (ch == '"' && c.state == htmlValueDouble) || (ch == '\'' && c.state == htmlValueSingle) {
				if c.open != nil {
					flush(i)
				}
				m.endValue()
				c.state = htmlInTag
			} else {
				m.valueText(ch)
			}
		case htmlValueUnquoted:
			switch {
			case ch == '>':
				if c.open != nil {
					flush(i)
				}
				m.endValue()
				m.closeTag()
			case isHTMLSpace(ch):
				if c.open != nil {
					flush(i)
				}
				m.endValue()
				c.state = htmlInTag
			default:
				m.valueText(ch)
			}
		case htmlRawText:
			// Raw text ends only at "</" and its element's name.
			if ch != '<' && bytes.IndexByte(c.tail[len(c.tail)-len(c.raw)-1:], '<') < 0 {
				// Nothing read since the last "<" can start the end tag: skip
				// to the next one.
				j := strings.IndexByte(t[i:], '<')
				if j < 0 {
					i = len(t)
					break
				}
				i += j
				ch = '<'
			}
			c.pushTail(lowerASCII(ch))
			if end := len(c.raw) + 2; c.tail[len(c.tail)-end] == '<' && c.tail[len(c.tail)-end+1] == '/' &&
				string(c.tail[len(c.tail)-len(c.raw):]) == c.raw {
				c.state = htmlTagName
				c.tag = c.raw
				c.closing = true
				c.raw = ""
			}
		case htmlComment:
			if len(c.opener) < 2 {
				c.opener += string(ch)
			}
			c.pushTail(ch)
			if ch == '>' && (c.opener != "--" || string(c.tail[len(c.tail)-3:]) == "-->") {
				c.state = htmlData
			}
		}
	}
	m.endName(t, len(t))
	flush(len(t))
}

// endName ends the tag or attribute name being read in t at i.
func (m *htmlMarker) endName(t string, i int) {
	if m.tagFrom >= 0 {
		m.ctx.tag += strings.ToLower(t[m.tagFrom:i])
		m.tagFrom = -1
	}
	if m.attrFrom >= 0 {
		m.ctx.attr += strings.ToLower(t[m.attrFrom:i])
		m.attrFrom = -1
	}
}

func (c *htmlCtx) pushTail(ch byte) {
	copy(c.tail[:], c.tail[1:])
	c.tail[len(c.tail)-1] = ch
}

func (m *htmlMarker) startAttr(i int) {
	m.ctx.state = htmlAttrName
	m.ctx.attr = ""
	m.attrFrom = i
	m.ctx.ambiguousName = false
}

func (m *htmlMarker) checkNamed(pos int) {
	if m.ctx.ambiguousName {
		m.fail(pos, "the attribute this value belongs to in <%s> depends on control flow; write the attribute name and its value in the same branch", m.ctx.tag)
	}
}

// valueText records literal text of a style value.
func (m *htmlMarker) valueText(ch byte) {
	if m.ctx.value == attrStyle {
		m.ctx.pushTail(lowerASCII(ch))
	}
}

func (m *htmlMarker) closeTag() {
	c := &m.ctx
	if !c.closing && rawTextElements[c.tag] {
		c.state = htmlRawText
		c.raw = c.tag
		c.tail = [16]byte{}
		return
	}
	c.state = htmlData
}

var rawTextElements = map[string]bool{"script": true, "style": true, "textarea": true, "title": true}

// join is the context after paths that end in ends, failing when they differ
// in a way that changes how a later output prints.
func (m *htmlMarker) join(ends []htmlCtx, pos int, what string) htmlCtx {
	out := ends[0]
	for _, c := range ends[1:] {
		j, ok := joinCtx(out, c)
		if !ok {
			m.fail(pos, "%s end in different HTML contexts (%s and %s); open and close tags and attribute values in the same branch", what, describeCtx(out), describeCtx(c))
			return out
		}
		out = j
	}
	return out
}

// joinCtx merges two contexts that print every later output the same way.
func joinCtx(a, b htmlCtx) (htmlCtx, bool) {
	tagish := func(s htmlState) bool {
		return s == htmlTagName || s == htmlInTag || s == htmlAttrName || s == htmlAfterAttrName
	}
	a.printedAttr = a.printedAttr || b.printedAttr
	// A path that ended inside a name continues it with whatever comes next.
	a.nameOpen = a.inName() || b.inName()
	switch {
	case a.state != b.state && !(tagish(a.state) && tagish(b.state)):
		return a, false
	case a.inTag() && a.closing != b.closing:
		return a, false
	case a.inTag() && a.tag != b.tag:
		// Elements may differ while neither is read as raw text; an unknown
		// element's src is read as an ordinary URL.
		if rawTextElements[a.tag] || rawTextElements[b.tag] {
			return a, false
		}
		a.tag = ""
	}
	switch a.state {
	case htmlData:
		return a, true
	case htmlRawText:
		return a, a.raw == b.raw
	case htmlComment:
		return a, (a.opener == "--") == (b.opener == "--")
	case htmlBeforeValue:
		return a, classifyAttr(a.tag, a.attr) == classifyAttr(b.tag, b.attr)
	case htmlValueDouble, htmlValueSingle, htmlValueUnquoted:
		return a, a.value == b.value && (a.value == attrPlain || a.valuePos == b.valuePos)
	}
	// In a tag, between or in attribute names.
	named := func(c htmlCtx) bool { return c.state == htmlAttrName || c.state == htmlAfterAttrName }
	switch {
	case named(a) && named(b):
		a.ambiguousName = a.ambiguousName || b.ambiguousName || classifyAttr(a.tag, a.attr) != classifyAttr(b.tag, b.attr)
	case named(a) || named(b):
		if !named(a) {
			a.attr = b.attr
		}
		a.ambiguousName = a.ambiguousName || b.ambiguousName || classifyAttr(a.tag, a.attr) != attrPlain
	default:
		a.state = htmlInTag
		return a, true
	}
	a.state = htmlAfterAttrName
	return a, true
}

func describeCtx(c htmlCtx) string {
	switch c.state {
	case htmlData:
		return "outside any tag"
	case htmlRawText:
		return fmt.Sprintf("inside <%s>", c.raw)
	case htmlComment:
		return "inside a comment"
	case htmlBeforeValue, htmlValueDouble, htmlValueSingle, htmlValueUnquoted:
		return fmt.Sprintf("in the %s attribute of <%s>", c.attr, c.tag)
	}
	return fmt.Sprintf("inside the <%s> tag", c.tag)
}

func keepTail(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}

func isASCIILetter(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isASCIIDigit(c byte) bool  { return c >= '0' && c <= '9' }
func isHTMLSpace(c byte) bool   { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' }
func lowerASCII(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

// stripTags is the text of HTML s: its tags and comments removed. What remains
// is escaped where it prints, so this only keeps markup from showing as text.
func stripTags(s string) string {
	if strings.IndexByte(s, '<') < 0 {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		j := strings.IndexByte(s[i:], '<')
		if j < 0 {
			b.WriteString(s[i:])
			break
		}
		b.WriteString(s[i : i+j])
		i += j
		if i+1 >= len(s) || !(isASCIILetter(s[i+1]) || s[i+1] == '/' || s[i+1] == '!' || s[i+1] == '?') {
			b.WriteByte('<')
			i++
			continue
		}
		if strings.HasPrefix(s[i:], "<!--") {
			end := strings.Index(s[i+4:], "-->")
			if end < 0 {
				break
			}
			i += 4 + end + 3
			continue
		}
		// A tag runs to the first ">" outside a quoted attribute value.
		var quote byte
		k := i + 1
		for ; k < len(s); k++ {
			c := s[k]
			switch {
			case quote != 0:
				if c == quote {
					quote = 0
				}
			case c == '"' || c == '\'':
				quote = c
			case c == '>':
				goto closed
			}
		}
	closed:
		i = k + 1
	}
	return b.String()
}
