package vascula

import (
	"html"
	"strings"
)

// URL attributes.
//
// Escaping keeps a value inside its attribute, but a value in a URL still
// chooses the URL's scheme, and javascript: runs when followed. Like Go's
// html/template, a URL-bearing attribute whose value has any printed part (an
// output tag, {% cycle %}, {% render %} or host tag) is checked; one written
// entirely in the template is the author's and is not. The check reads the
// whole value as the browser does once the attribute is complete: literal
// text and printed parts together, character references decoded, surrounding
// control characters and spaces ignored, tabs and line breaks removed, and the
// scheme compared without case. The value must be a relative URL (a path,
// query or fragment), or use http, https, mailto or tel; otherwise the whole
// value renders as "#".
//
// In srcset each image candidate is checked, read as the HTML standard reads
// srcset (a URL is a run of non-space characters, so commas inside it are
// kept); an unsafe candidate URL renders as "#" and keeps its descriptors. In
// a style attribute each url() argument is checked, and an unsafe one renders
// as "#". Image data URLs are allowed only in the src of img and source and in
// style url().

// attrKind is what an attribute's value holds.
type attrKind uint8

const (
	attrPlain attrKind = iota
	attrURL
	// attrImageSource is the src of an image element, which may also be an
	// image data URL.
	attrImageSource
	attrSrcset
	attrStyle
)

// urlAttr is one URL-bearing attribute value in a template. dynamic is set at
// compile time when something printed stands in it; only then is it checked.
type urlAttr struct {
	kind     attrKind
	unquoted bool
	dynamic  bool
}

// urlAttrs hold one URL each (HTML, SVG and the obsolete attributes browsers
// still load).
var urlAttrs = map[string]bool{
	"action": true, "archive": true, "background": true, "cite": true, "classid": true,
	"codebase": true, "data": true, "formaction": true, "href": true, "icon": true,
	"longdesc": true, "manifest": true, "ping": true, "poster": true, "profile": true,
	"src": true, "usemap": true, "xmlns": true,
}

// classifyAttr names what the value of attribute name on element tag holds.
// As in html/template, "data-" is ignored ("data-src" is a source for scripts
// that copy it into src), a namespace prefix is ignored except xmlns, and an
// unknown name containing "src", "uri" or "url" is taken to be a URL.
func classifyAttr(tag, name string) attrKind {
	if name == "style" {
		return attrStyle
	}
	name = strings.TrimPrefix(name, "data-")
	if prefix, local, ok := strings.Cut(name, ":"); ok {
		if prefix == "xmlns" {
			return attrURL
		}
		name = local
	}
	switch {
	case name == "srcset":
		return attrSrcset
	case name == "src" && (tag == "img" || tag == "source"):
		return attrImageSource
	case urlAttrs[name]:
		return attrURL
	case strings.Contains(name, "src") || strings.Contains(name, "uri") || strings.Contains(name, "url"):
		return attrURL
	}
	return attrPlain
}

// plainAttrName reports whether s, printed where an attribute goes, is empty
// or one attribute name that holds no script, style or URL.
func plainAttrName(s string) bool {
	if s == "" {
		return true
	}
	if !isASCIILetter(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if c := s[i]; !isASCIILetter(c) && !isASCIIDigit(c) && c != '-' && c != '_' {
			return false
		}
	}
	name := strings.ToLower(s)
	return !strings.HasPrefix(name, "on") && classifyAttr("", name) == attrPlain
}

// opensCSSURL reports whether style text ends inside "url(", before its
// argument: url( then optional spaces and an optional quote.
func opensCSSURL(tail string) bool {
	t := strings.TrimRight(tail, " \t\n\r\f")
	t = strings.TrimSuffix(strings.TrimSuffix(t, `"`), "'")
	t = strings.TrimRight(t, " \t\n\r\f")
	return strings.HasSuffix(t, "url(")
}

// safeURL reports whether u, as text the browser reads, is relative or uses an
// allowed scheme. image allows image data URLs.
func safeURL(u string, image bool) bool {
	u = strings.TrimFunc(u, func(r rune) bool { return r <= ' ' })
	if strings.ContainsAny(u, "\t\n\r") {
		u = strings.NewReplacer("\t", "", "\n", "", "\r", "").Replace(u)
	}
	i := strings.IndexAny(u, ":/?#")
	if i < 0 || u[i] != ':' {
		return true
	}
	switch strings.ToLower(u[:i]) {
	case "http", "https", "mailto", "tel":
		return true
	case "data":
		return image && hasPrefixFold(u[i+1:], "image/")
	}
	return false
}

func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

// checkAttrValue returns the markup for a URL-bearing attribute value that
// rendered as printed: printed itself when it is safe, a replacement otherwise.
func checkAttrValue(a *urlAttr, printed string) string {
	text := html.UnescapeString(printed)
	var out string
	switch a.kind {
	case attrURL, attrImageSource:
		if safeURL(text, a.kind == attrImageSource) {
			return printed
		}
		out = "#"
	case attrSrcset:
		var changed bool
		if out, changed = checkSrcset(text); !changed {
			return printed
		}
	case attrStyle:
		var changed bool
		if out, changed = checkStyle(text); !changed {
			return printed
		}
	default:
		return printed
	}
	if a.unquoted {
		return unquotedEscaper.Replace(out)
	}
	return html.EscapeString(out)
}

// checkSrcset replaces each unsafe candidate URL in a srcset by "#", keeping
// descriptors. It follows the HTML standard's srcset parser: candidates are
// separated by whitespace and commas, a URL is a run of non-whitespace (its
// trailing commas end the candidate), and descriptors run to the next comma
// outside parentheses.
func checkSrcset(s string) (string, bool) {
	var b strings.Builder
	last, changed := 0, false
	for i := 0; ; {
		for i < len(s) && (isHTMLSpace(s[i]) || s[i] == ',') {
			i++
		}
		if i >= len(s) {
			break
		}
		start := i
		for i < len(s) && !isHTMLSpace(s[i]) {
			i++
		}
		url := strings.TrimRight(s[start:i], ",")
		if !safeURL(url, false) {
			b.WriteString(s[last:start])
			b.WriteString("#")
			last = start + len(url)
			changed = true
		}
		if start+len(url) < i {
			continue // a URL ending in a comma has no descriptors
		}
	descriptors:
		for parens := false; i < len(s); i++ {
			switch c := s[i]; {
			case parens:
				parens = c != ')'
			case c == '(':
				parens = true
			case c == ',':
				i++
				break descriptors
			}
		}
	}
	if !changed {
		return s, false
	}
	b.WriteString(s[last:])
	return b.String(), true
}

// checkStyle replaces each unsafe url() argument in style text by "#". CSS
// escapes can spell url( or a scheme; text with escapes that names an unsafe
// URL once they are decoded renders as an empty style.
func checkStyle(s string) (string, bool) {
	if strings.IndexByte(s, '\\') >= 0 {
		if _, unsafe := rewriteCSSURLs(cssUnescape(s)); unsafe {
			return "", true
		}
	}
	return rewriteCSSURLs(s)
}

func rewriteCSSURLs(s string) (string, bool) {
	lower := strings.ToLower(s)
	var b strings.Builder
	last, changed := 0, false
	for i := 0; ; {
		j := strings.Index(lower[i:], "url(")
		if j < 0 {
			break
		}
		start := i + j + len("url(")
		for start < len(s) && isHTMLSpace(s[start]) {
			start++
		}
		end := start
		if start < len(s) && (s[start] == '"' || s[start] == '\'') {
			start++
			end = start
			for end < len(s) && s[end] != s[start-1] {
				end++
			}
		} else {
			for end < len(s) && s[end] != ')' && !isHTMLSpace(s[end]) {
				end++
			}
		}
		if !safeURL(s[start:end], true) {
			b.WriteString(s[last:start])
			b.WriteString("#")
			last = end
			changed = true
		}
		i = end
	}
	if !changed {
		return s, false
	}
	b.WriteString(s[last:])
	return b.String(), true
}

// cssUnescape decodes CSS escapes: a backslash and up to six hex digits (and
// one following space), an escaped line break (removed), or an escaped
// character.
func cssUnescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 == len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		n, r := 0, rune(0)
		for n < 6 && i+n < len(s) && isHexDigit(s[i+n]) {
			r = r*16 + rune(hexValue(s[i+n]))
			n++
		}
		switch {
		case n > 0:
			if r == 0 || r > 0x10FFFF || (r >= 0xD800 && r <= 0xDFFF) {
				r = '�'
			}
			b.WriteRune(r)
			i += n
			if i < len(s) && isHTMLSpace(s[i]) {
				if s[i] == '\r' && i+1 < len(s) && s[i+1] == '\n' {
					i++
				}
			} else {
				i--
			}
		case s[i] == '\n' || s[i] == '\f':
		case s[i] == '\r':
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func isHexDigit(c byte) bool {
	return isASCIIDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func hexValue(c byte) int {
	switch {
	case isASCIIDigit(c):
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	}
	return int(c-'A') + 10
}

// cssURLEscaper percent-encodes what would end a CSS url() argument or the
// string around it.
var cssURLEscaper = strings.NewReplacer(
	" ", "%20", "\t", "%09", "\n", "%0A", "\r", "%0D", "\f", "%0C",
	`"`, "%22", "'", "%27", "(", "%28", ")", "%29", `\`, "%5C",
)

// unquotedEscaper escapes text for an unquoted attribute value: markup
// characters, and everything that would end the value or start another
// attribute (whitespace, quotes, backtick, =, / and >). The browser decodes the
// references, so the value reads as the text.
var unquotedEscaper = strings.NewReplacer(
	"&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;", "'", "&#39;", "`", "&#96;",
	"=", "&#61;", "/", "&#47;", " ", "&#32;", "\t", "&#9;", "\n", "&#10;", "\r", "&#13;",
	"\f", "&#12;", "\v", "&#11;", "\x00", "&#xFFFD;",
)
