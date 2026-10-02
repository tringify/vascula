package vascula

import (
	"fmt"
	"strings"
	"unicode"
)

type tokenType int

const (
	tEOF  tokenType = iota
	tText           // literal markup between tags

	tOutputOpen  // {{
	tOutputClose // }}
	tTagOpen     // {%
	tTagClose    // %}

	tIdent  // names + keywords (if, for, assign, ...)
	tString // 'x' or "x"
	tNumber // 12, 3.14, -1
	tDot    // .
	tLBrack // [
	tRBrack // ]
	tPipe   // |
	tColon  // :
	tComma  // ,
	tEq     // ==
	tNeq    // !=
	tGt     // >
	tLt     // <
	tGte    // >=
	tLte    // <=
	tAssign // = (in assign / named args)
	tLParen // (
	tRParen // )
	tRange  // ..
)

type token struct {
	typ  tokenType
	val  string
	pos  int  // byte offset, for error messages
	trim bool // whitespace-control marker on the closing side ({{- / -}} / {%- / -%})
}

func (t token) String() string { return describeToken(t) }

var tokenNames = map[tokenType]string{
	tEOF: "end of template", tText: "text", tOutputOpen: "{{", tOutputClose: "}}",
	tTagOpen: "{%", tTagClose: "%}", tIdent: "name", tString: "string", tNumber: "number",
	tDot: ".", tLBrack: "[", tRBrack: "]", tPipe: "|", tColon: ":", tComma: ",",
	tEq: "==", tNeq: "!=", tGt: ">", tLt: "<", tGte: ">=", tLte: "<=", tAssign: "=",
	tLParen: "(", tRParen: ")", tRange: "..",
}

func (t tokenType) String() string {
	if name, ok := tokenNames[t]; ok {
		if len(name) <= 2 {
			return "\"" + name + "\""
		}
		return name
	}
	return "token"
}

// describeToken names a token for an error message, with its text when it
// carries one: name "producs", string "x", number 12.
func describeToken(t token) string {
	switch t.typ {
	case tIdent, tString, tNumber:
		return fmt.Sprintf("%v %q", t.typ, t.val)
	}
	return t.typ.String()
}

// lex tokenizes src with the default token limit. The parser checks grammar.
func lex(src string) ([]token, error) {
	return lexWithLimit(src, defaultMaxTokens)
}

func lexWithLimit(src string, maxTokens int) ([]token, error) {
	l := &lexer{src: src, maxTokens: maxTokens}
	if err := l.run(); err != nil {
		return nil, err
	}
	return l.tokens, l.err
}

type lexer struct {
	src             string
	pos             int
	tokens          []token
	pendingLeftTrim bool // set by -}} / -%%}; trims the next TEXT token's left side
	maxTokens       int
	err             error
}

func (l *lexer) emit(t tokenType, val string, pos int) {
	if l.err != nil {
		return
	}
	if len(l.tokens) >= l.maxTokens {
		l.err = atOffset(pos, "compile budget exceeded: too many tokens")
		return
	}
	l.tokens = append(l.tokens, token{typ: t, val: val, pos: pos})
}

// emitText emits literal markup, honoring a pending left-trim from -}} / -%}.
// Empty results are dropped so trims don't leave stray nodes.
func (l *lexer) emitText(val string, pos int) {
	if l.pendingLeftTrim {
		val = strings.TrimLeftFunc(val, unicode.IsSpace)
		l.pendingLeftTrim = false
	}
	if val != "" {
		l.emit(tText, val, pos)
	}
}

func (l *lexer) run() error {
	for l.pos < len(l.src) {
		// Find the next tag/output opener.
		next := l.indexOfOpener(l.pos)
		if next < 0 {
			if l.pos < len(l.src) {
				l.emitText(l.src[l.pos:], l.pos)
			}
			break
		}
		if next > l.pos {
			l.emitText(l.src[l.pos:next], l.pos)
		}
		l.pos = next

		// Inline comment {# ... #}: discarded entirely — an unterminated one is a
		// compile error (silent truncation would eat the rest of the template).
		if l.src[l.pos+1] == '#' {
			start := l.pos
			l.pos += 2
			if l.pos < len(l.src) && l.src[l.pos] == '-' {
				l.pos++
				l.trimPrecedingText()
			}
			end := strings.Index(l.src[l.pos:], "#}")
			if end < 0 {
				return atOffset(start, "unterminated {# comment")
			}
			if end > 0 && l.src[l.pos+end-1] == '-' {
				l.pendingLeftTrim = true
			}
			l.pos += end + 2
			continue
		}

		isOutput := l.src[l.pos+1] == '{'
		open, closeSeq := "{{", "}}"
		openTok, closeTok := tOutputOpen, tOutputClose
		if !isOutput {
			open, closeSeq = "{%", "%}"
			openTok, closeTok = tTagOpen, tTagClose
		}

		// Whitespace control on the open side: {{- / {%-
		openPos := l.pos
		l.pos += len(open)
		leftTrim := l.pos < len(l.src) && l.src[l.pos] == '-'
		if leftTrim {
			l.pos++
			l.trimPrecedingText()
		}
		l.emit(openTok, open, openPos)
		if l.err != nil {
			return l.err
		}

		// comment blocks swallow their body until the matching end tag; raw blocks are rejected here.
		if !isOutput {
			if handled, err := l.maybeComment(); err != nil {
				return err
			} else if handled {
				continue
			}
		}

		if err := l.lexInner(closeSeq, closeTok); err != nil {
			return err
		}
	}
	l.emit(tEOF, "", l.pos)
	return nil
}

// indexOfOpener returns the offset of the next {{ or {% at/after from, or -1.
func (l *lexer) indexOfOpener(from int) int {
	for i := from; i+1 < len(l.src); i++ {
		if l.src[i] == '{' && (l.src[i+1] == '{' || l.src[i+1] == '%' || l.src[i+1] == '#') {
			return i
		}
	}
	return -1
}

// trimPrecedingText removes trailing whitespace from the last TEXT token (the
// {{- / {%- whitespace-control behavior).
func (l *lexer) trimPrecedingText() {
	if n := len(l.tokens); n > 0 && l.tokens[n-1].typ == tText {
		l.tokens[n-1].val = strings.TrimRightFunc(l.tokens[n-1].val, unicode.IsSpace)
	}
}

// lexInner tokenizes expression tokens until the given close sequence.
func (l *lexer) lexInner(closeSeq string, closeTok tokenType) error {
	for {
		if l.err != nil {
			return l.err
		}
		l.skipSpaces()
		if l.pos >= len(l.src) {
			return atOffset(l.pos, "unterminated %q", closeSeq)
		}
		// Right whitespace control: -}} / -%}
		if l.src[l.pos] == '-' && strings.HasPrefix(l.src[l.pos+1:], closeSeq) {
			l.pos++ // consume '-'
			l.trimFollowingText(true)
		}
		if strings.HasPrefix(l.src[l.pos:], closeSeq) {
			pos := l.pos
			l.pos += len(closeSeq)
			l.emit(closeTok, closeSeq, pos)
			return nil
		}
		if err := l.lexToken(); err != nil {
			return err
		}
	}
}

// trimFollowingText flags that the next TEXT token should be left-trimmed. We
// record intent by stashing a sentinel; applied when the next text is emitted.
func (l *lexer) trimFollowingText(_ bool) { l.pendingLeftTrim = true }

func (l *lexer) lexToken() error {
	c := l.src[l.pos]
	start := l.pos
	switch {
	case c == '.' && l.peek(1) == '.':
		l.pos += 2
		l.emit(tRange, "..", start)
	case c == '.':
		l.pos++
		l.emit(tDot, ".", start)
	case c == '(':
		l.pos++
		l.emit(tLParen, "(", start)
	case c == ')':
		l.pos++
		l.emit(tRParen, ")", start)
	case c == '[':
		l.pos++
		l.emit(tLBrack, "[", start)
	case c == ']':
		l.pos++
		l.emit(tRBrack, "]", start)
	case c == '|':
		l.pos++
		l.emit(tPipe, "|", start)
	case c == ':':
		l.pos++
		l.emit(tColon, ":", start)
	case c == ',':
		l.pos++
		l.emit(tComma, ",", start)
	case c == '\'' || c == '"':
		return l.lexString(c)
	case c == '=':
		if l.peek(1) == '=' {
			l.pos += 2
			l.emit(tEq, "==", start)
		} else {
			l.pos++
			l.emit(tAssign, "=", start)
		}
	case c == '!':
		if l.peek(1) == '=' {
			l.pos += 2
			l.emit(tNeq, "!=", start)
		} else {
			return atOffset(start, "unexpected '!'")
		}
	case c == '>':
		if l.peek(1) == '=' {
			l.pos += 2
			l.emit(tGte, ">=", start)
		} else {
			l.pos++
			l.emit(tGt, ">", start)
		}
	case c == '<':
		if l.peek(1) == '=' {
			l.pos += 2
			l.emit(tLte, "<=", start)
		} else {
			l.pos++
			l.emit(tLt, "<", start)
		}
	case c == '-' || (c >= '0' && c <= '9'):
		return l.lexNumber()
	case isIdentStart(c):
		return l.lexIdent()
	default:
		return atOffset(start, "unexpected char %q", string(c))
	}
	return nil
}

func (l *lexer) lexString(quote byte) error {
	start := l.pos
	l.pos++ // opening quote
	var b strings.Builder
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if c == '\\' && l.pos+1 < len(l.src) {
			b.WriteByte(l.src[l.pos+1])
			l.pos += 2
			continue
		}
		if c == quote {
			l.pos++
			l.emit(tString, b.String(), start)
			return nil
		}
		b.WriteByte(c)
		l.pos++
	}
	return atOffset(start, "unterminated string")
}

func (l *lexer) lexNumber() error {
	start := l.pos
	if l.src[l.pos] == '-' {
		l.pos++
	}
	// One decimal point, and only when a digit follows, so "1..5" is a range.
	seenPoint := false
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if unicode.IsDigit(rune(c)) {
			l.pos++
			continue
		}
		if c == '.' && !seenPoint && l.pos+1 < len(l.src) && unicode.IsDigit(rune(l.src[l.pos+1])) {
			seenPoint = true
			l.pos++
			continue
		}
		break
	}
	l.emit(tNumber, l.src[start:l.pos], start)
	return nil
}

func (l *lexer) lexIdent() error {
	start := l.pos
	for l.pos < len(l.src) && isIdentPart(l.src[l.pos]) {
		l.pos++
	}
	l.emit(tIdent, l.src[start:l.pos], start)
	return nil
}

// maybeComment handles {% comment %}...{% endcomment %}: its body is not tokenized. Returns true if it
// consumed a full block (including the closing tag).
//
// {% raw %} is REJECTED here at compile: like the `raw` filter, a raw block is an author-facing way to
// emit literal markup outside the auto-escape contract. There is no author escape hatch — trusted HTML
// enters only as host-vouched SafeHTML. (comment stays: it hides its body, it does not emit markup.)
func (l *lexer) maybeComment() (bool, error) {
	save := l.pos
	l.skipSpaces()
	word := l.peekIdent()
	switch word {
	case "raw":
		return false, atOffset(l.pos, "the {%% raw %%} block is not available: output is always escaped")
	case "comment":
		open := l.tokens[len(l.tokens)-1].pos
		_, end, err := l.scanUntilEndTag("endcomment")
		if err != nil {
			return false, atOffset(open, "{%% comment %%} is never closed with {%% endcomment %%}")
		}
		l.tokens = l.tokens[:len(l.tokens)-1] // drop the {%
		l.pos = end
		return true, nil
	}
	l.pos = save
	return false, nil
}

// scanUntilEndTag returns the literal body up to (and consuming) {% endX %},
// returning the body and the offset just past the closing tag.
func (l *lexer) scanUntilEndTag(endWord string) (body string, end int, err error) {
	// advance past the opening keyword's %} first
	close := strings.Index(l.src[l.pos:], "%}")
	if close < 0 {
		return "", 0, fmt.Errorf("unterminated %q tag", endWord)
	}
	bodyStart := l.pos + close + 2
	rest := l.src[bodyStart:]
	// find {% ... endWord ... %}
	idx := indexEndTag(rest, endWord)
	if idx < 0 {
		return "", 0, fmt.Errorf("missing {%% %s %%}", endWord)
	}
	body = rest[:idx]
	after := rest[idx:]
	closeIdx := strings.Index(after, "%}")
	return body, bodyStart + idx + closeIdx + 2, nil
}

func (l *lexer) skipSpaces() {
	for l.pos < len(l.src) && (l.src[l.pos] == ' ' || l.src[l.pos] == '\t' || l.src[l.pos] == '\n' || l.src[l.pos] == '\r') {
		l.pos++
	}
}

func (l *lexer) peek(n int) byte {
	if l.pos+n < len(l.src) {
		return l.src[l.pos+n]
	}
	return 0
}

func (l *lexer) peekIdent() string {
	i := l.pos
	for i < len(l.src) && isIdentPart(l.src[i]) {
		i++
	}
	return l.src[l.pos:i]
}

func isIdentStart(c byte) bool { return c == '_' || unicode.IsLetter(rune(c)) }
func isIdentPart(c byte) bool {
	return c == '_' || c == '-' || unicode.IsLetter(rune(c)) || unicode.IsDigit(rune(c))
}

// indexEndTag finds the byte offset of the next "{%" that introduces endWord.
func indexEndTag(s, endWord string) int {
	from := 0
	for {
		idx := strings.Index(s[from:], "{%")
		if idx < 0 {
			return -1
		}
		abs := from + idx
		inner := s[abs+2:]
		trimmed := strings.TrimLeft(inner, "-")
		trimmed = strings.TrimLeft(trimmed, " \t\r\n")
		// Require a WORD BOUNDARY after the keyword so {% endcommentary %} doesn't match endcomment — the next
		// char must end the identifier (whitespace, %, -, or end of string), not an ident char.
		if strings.HasPrefix(trimmed, endWord) {
			rest := trimmed[len(endWord):]
			if rest == "" || !isIdentPart(rest[0]) {
				return abs
			}
		}
		from = abs + 2
	}
}
