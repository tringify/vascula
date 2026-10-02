package vascula

import "strings"

// Output context.
//
// The json filter's output is safe inside <script> (the encoder escapes <, >
// and &) but not inside an HTML tag: its quotes end an attribute value, and
// the spaces of a string value then start new attributes. At compile time the
// template's literal markup is scanned in source order, and every output tag
// that sits inside a tag (an attribute value, or where an attribute would go)
// is marked. There JSON text is HTML-escaped on write, so `{{ x | json }}` is
// as safe as `{{ x | json | escape }}`, which renders exactly as before.
//
// The scan is lexical and follows source order through if/for/case bodies; a
// template that opens a tag in one branch and closes it in another is read in
// the order written.

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

type htmlScanner struct {
	state htmlState
	// tag is the name of the tag being read; closing marks </name.
	tag     strings.Builder
	closing bool
	// raw is the element whose text ends only at its end tag (script, style).
	raw string
	// pending holds the last characters read in raw text or a comment, enough
	// to recognise the end marker across text nodes.
	pending string
	// opener is what followed "<!": "--" starts a comment that only "-->"
	// ends; anything else (a doctype) ends at ">".
	opener string
}

func (s *htmlScanner) inTag() bool {
	switch s.state {
	case htmlTagName, htmlInTag, htmlAttrName, htmlAfterAttrName, htmlBeforeValue,
		htmlValueDouble, htmlValueSingle, htmlValueUnquoted:
		return true
	}
	return false
}

// output advances past an output tag: one standing where an attribute value
// starts is that value.
func (s *htmlScanner) output() {
	if s.state == htmlBeforeValue {
		s.state = htmlValueUnquoted
	}
}

func (s *htmlScanner) text(t string) {
	for i := 0; i < len(t); i++ {
		c := t[i]
		switch s.state {
		case htmlData:
			if c == '<' {
				s.state = htmlTagName
				s.tag.Reset()
				s.closing = false
			}
		case htmlTagName:
			switch {
			case c == '/' && s.tag.Len() == 0 && !s.closing:
				s.closing = true
			case c == '!' && s.tag.Len() == 0 && !s.closing:
				s.state = htmlComment
				s.pending = ""
				s.opener = ""
			case isASCIILetter(c) || (s.tag.Len() > 0 && (isASCIIDigit(c) || c == '-')):
				s.tag.WriteByte(lowerASCII(c))
			case s.tag.Len() == 0:
				// "<" not followed by a letter, "/" or "!" ("a < b") is text.
				s.state = htmlData
			case c == '>':
				s.closeTag()
			default:
				s.state = htmlInTag
			}
		case htmlInTag:
			switch {
			case c == '>':
				s.closeTag()
			case isHTMLSpace(c) || c == '/':
			default:
				s.state = htmlAttrName
			}
		case htmlAttrName:
			switch {
			case c == '>':
				s.closeTag()
			case c == '=':
				s.state = htmlBeforeValue
			case isHTMLSpace(c):
				s.state = htmlAfterAttrName
			}
		case htmlAfterAttrName:
			switch {
			case c == '>':
				s.closeTag()
			case c == '=':
				s.state = htmlBeforeValue
			case isHTMLSpace(c) || c == '/':
			default:
				s.state = htmlAttrName
			}
		case htmlBeforeValue:
			switch {
			case c == '"':
				s.state = htmlValueDouble
			case c == '\'':
				s.state = htmlValueSingle
			case c == '>':
				s.closeTag()
			case isHTMLSpace(c):
			default:
				s.state = htmlValueUnquoted
			}
		case htmlValueDouble:
			if c == '"' {
				s.state = htmlInTag
			}
		case htmlValueSingle:
			if c == '\'' {
				s.state = htmlInTag
			}
		case htmlValueUnquoted:
			switch {
			case c == '>':
				s.closeTag()
			case isHTMLSpace(c):
				s.state = htmlInTag
			}
		case htmlRawText:
			s.pending = keepTail(s.pending+string(lowerASCII(c)), len(s.raw)+2)
			if strings.HasSuffix(s.pending, "</"+s.raw) {
				s.state = htmlTagName
				s.tag.Reset()
				s.tag.WriteString(s.raw)
				s.closing = true
				s.raw = ""
			}
		case htmlComment:
			if len(s.opener) < 2 {
				s.opener += string(c)
			}
			s.pending = keepTail(s.pending+string(c), 3)
			if c == '>' && (s.opener != "--" || (len(s.pending) == 3 && s.pending == "-->")) {
				s.state = htmlData
			}
		}
	}
}

func (s *htmlScanner) closeTag() {
	name := s.tag.String()
	if !s.closing && (name == "script" || name == "style" || name == "textarea" || name == "title") {
		s.state = htmlRawText
		s.raw = name
		s.pending = ""
		return
	}
	s.state = htmlData
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

// markOutputContexts records, for every output tag, whether it sits inside an
// HTML tag.
func markOutputContexts(nodes []node) []node {
	s := &htmlScanner{}
	return s.mark(nodes)
}

func (s *htmlScanner) mark(nodes []node) []node {
	out := make([]node, len(nodes))
	for i, n := range nodes {
		switch x := n.(type) {
		case textNode:
			s.text(x.text)
		case outputNode:
			x.inTag = s.inTag()
			s.output()
			n = x
		case ifNode:
			for b := range x.branches {
				x.branches[b].body = s.mark(x.branches[b].body)
			}
			x.elseBody = s.mark(x.elseBody)
			n = x
		case forNode:
			x.body = s.mark(x.body)
			x.elseBody = s.mark(x.elseBody)
			n = x
		case caseNode:
			for w := range x.whens {
				x.whens[w].body = s.mark(x.whens[w].body)
			}
			x.elseBody = s.mark(x.elseBody)
			n = x
		case captureNode:
			x.body = s.mark(x.body)
			n = x
		case formNode:
			x.body = s.mark(x.body)
			n = x
		}
		out[i] = n
	}
	return out
}
