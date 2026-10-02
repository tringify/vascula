package vascula

import (
	"fmt"
	"strconv"
	"strings"
)

// parseTemplate lexes + parses source into a node list. Errors carry the byte
// offset so a bad template fails compilation with a precise location.
func parseTemplate(src string, opts CompileOptions) ([]node, error) {
	if len(src) > orDefault(opts.MaxSourceBytes, defaultMaxSourceBytes) {
		return nil, fmt.Errorf("compile budget exceeded: source too large")
	}
	toks, err := lexWithLimit(src, orDefault(opts.MaxTokens, defaultMaxTokens))
	if err != nil {
		return nil, err
	}
	if err := validateTags(opts.Tags); err != nil {
		return nil, err
	}
	reserved := make(map[string]bool, len(opts.Reserved))
	for _, name := range opts.Reserved {
		reserved[name] = true
	}
	p := &parser{toks: toks, forms: opts.Forms, maxNesting: orDefault(opts.MaxNesting, defaultMaxNesting), tags: opts.Tags, reserved: reserved}
	nodes, stop, err := p.parseBlock(nil)
	if err != nil {
		return nil, err
	}
	if stop != "" {
		return nil, atOffset(p.peek().pos, "unexpected {%% %s %%} with no opening tag", stop)
	}
	return nodes, nil
}

type parser struct {
	forms      map[string]FormDefinition
	toks       []token
	pos        int
	nesting    int
	maxNesting int
	tags       map[string]TagDefinition
	reserved   map[string]bool
	// openPos is the {% of the tag being parsed; block parsers keep it so an
	// unclosed block reports where it was opened.
	openPos int
	// cycleSeq numbers {% cycle %} nodes so the evaluator keys rotation state
	// per tag occurrence.
	cycleSeq int
}

func (p *parser) peek() token { return p.toks[p.pos] }
func (p *parser) next() token { t := p.toks[p.pos]; p.pos++; return t }
func (p *parser) atEOF() bool { return p.toks[p.pos].typ == tEOF }

func (p *parser) expect(tt tokenType) (token, error) {
	if p.peek().typ != tt {
		return token{}, atOffset(p.peek().pos, "expected %v, found %s", tt, describeToken(p.peek()))
	}
	return p.next(), nil
}

// parseBlock parses nodes until EOF or a {% tag %} whose keyword is in stops. On
// a stop it returns the keyword WITHOUT consuming the tag, so the caller (the
// opening construct) can consume and continue. stops==nil parses to EOF.
func (p *parser) parseBlock(stops map[string]bool) ([]node, string, error) {
	if err := p.enter(); err != nil {
		return nil, "", err
	}
	defer p.leave()
	var out []node
	for {
		t := p.peek()
		switch t.typ {
		case tEOF:
			return out, "", nil
		case tText:
			out = append(out, textNode{text: t.val})
			p.next()
		case tOutputOpen:
			n, err := p.parseOutput()
			if err != nil {
				return nil, "", err
			}
			out = append(out, n)
		case tTagOpen:
			kw := p.peekTagKeyword()
			if stops != nil && stops[kw] {
				return out, kw, nil
			}
			n, err := p.parseTag()
			if err != nil {
				return nil, "", err
			}
			if n != nil { // comment already consumed by the lexer; nil-safe
				out = append(out, n)
			}
		default:
			return nil, "", atOffset(t.pos, "unexpected %s", describeToken(t))
		}
	}
}

// peekTagKeyword returns the keyword of the upcoming {% tag %} without consuming.
func (p *parser) peekTagKeyword() string {
	if p.toks[p.pos].typ != tTagOpen {
		return ""
	}
	if p.pos+1 < len(p.toks) && p.toks[p.pos+1].typ == tIdent {
		return p.toks[p.pos+1].val
	}
	return ""
}

// consumeTagOpen consumes {% and the keyword ident, returning the keyword.
func (p *parser) consumeTagOpen() (string, error) {
	if _, err := p.expect(tTagOpen); err != nil {
		return "", err
	}
	kw, err := p.expect(tIdent)
	if err != nil {
		return "", err
	}
	return kw.val, nil
}

func (p *parser) parseOutput() (node, error) {
	if _, err := p.expect(tOutputOpen); err != nil {
		return nil, err
	}
	expr, err := p.parseExpr() // parseExpr already consumes a trailing filter pipeline
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tOutputClose); err != nil {
		return nil, err
	}
	return outputNode{expr: expr}, nil
}

func (p *parser) parseTag() (node, error) {
	p.openPos = p.peek().pos
	kw, err := p.consumeTagOpen()
	if err != nil {
		return nil, err
	}
	switch kw {
	case "if":
		return p.parseIf(false)
	case "unless":
		return p.parseIf(true)
	case "for":
		return p.parseFor()
	case "assign":
		return p.parseAssign()
	case "render":
		return p.parseRender()
	case "form":
		return p.parseForm()
	case "case":
		return p.parseCase()
	case "capture":
		return p.parseCapture()
	case "break":
		if _, err := p.expect(tTagClose); err != nil {
			return nil, err
		}
		return breakNode{}, nil
	case "continue":
		if _, err := p.expect(tTagClose); err != nil {
			return nil, err
		}
		return continueNode{}, nil
	case "cycle":
		return p.parseCycle()
	case "slot":
		// Unsupported syntax is rejected during compilation.
		return nil, atOffset(p.toks[p.pos-1].pos, "{%% slot %%} is not supported")
	case "else", "elsif", "when", "endif", "endunless", "endfor", "endcase", "endcapture", "endform":
		return nil, atOffset(p.openPos, "unexpected {%% %s %%} with no opening tag", kw)
	default:
		if def, ok := p.tags[kw]; ok {
			return p.parseHostTag(kw, def)
		}
		return nil, atOffset(p.toks[p.pos-1].pos, "unknown tag %q", kw)
	}
}

var ifStops = map[string]bool{"elsif": true, "else": true, "endif": true}
var formStops = map[string]bool{"endform": true}
var unlessStops = map[string]bool{"else": true, "endunless": true}

func (p *parser) parseIf(negate bool) (node, error) {
	open := p.openPos
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if negate {
		// unless renders when cond is FALSY → negate its truthiness (NOT cond == false,
		// which is wrong for nil/empty/"" etc.).
		cond = notExpr{inner: cond}
	}
	if _, err := p.expect(tTagClose); err != nil {
		return nil, err
	}
	stops := ifStops
	endWord := "endif"
	if negate {
		stops = unlessStops
		endWord = "endunless"
	}
	body, stop, err := p.parseBlock(stops)
	if err != nil {
		return nil, err
	}
	branches := []ifBranch{{cond: cond, body: body}}

	for stop == "elsif" {
		if _, err := p.consumeTagOpen(); err != nil {
			return nil, err
		}
		c, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tTagClose); err != nil {
			return nil, err
		}
		var b []node
		b, stop, err = p.parseBlock(stops)
		if err != nil {
			return nil, err
		}
		branches = append(branches, ifBranch{cond: c, body: b})
	}

	var elseBody []node
	if stop == "else" {
		if _, err := p.consumeTagOpen(); err != nil {
			return nil, err
		}
		if _, err := p.expect(tTagClose); err != nil {
			return nil, err
		}
		elseBody, stop, err = p.parseBlock(map[string]bool{endWord: true})
		if err != nil {
			return nil, err
		}
	}
	if stop != endWord {
		opener := "if"
		if negate {
			opener = "unless"
		}
		return nil, atOffset(open, "{%% %s %%} is never closed with {%% %s %%}", opener, endWord)
	}
	if _, err := p.consumeTagOpen(); err != nil {
		return nil, err
	}
	if _, err := p.expect(tTagClose); err != nil {
		return nil, err
	}
	return ifNode{branches: branches, elseBody: elseBody}, nil
}

var forStops = map[string]bool{"else": true, "endfor": true}

func (p *parser) parseFor() (node, error) {
	open := p.openPos
	varTok, err := p.expect(tIdent)
	if err != nil {
		return nil, err
	}
	if p.reservedBinding(varTok.val) {
		return nil, atOffset(varTok.pos, "for variable %q is reserved", varTok.val)
	}
	in, err := p.expect(tIdent)
	if err != nil {
		return nil, err
	}
	if in.val != "in" {
		return nil, atOffset(in.pos, "expected 'in' in for loop")
	}
	coll, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	fn := forNode{varName: varTok.val, coll: coll}

	// optional modifiers: limit: N | offset: N | reversed
	for p.peek().typ == tIdent {
		mod := p.next().val
		switch mod {
		case "limit", "offset":
			if _, err := p.expect(tColon); err != nil {
				return nil, err
			}
			e, err := p.parseExpr() // allow a filter pipeline (e.g. limit: settings.n | plus: 1)
			if err != nil {
				return nil, err
			}
			if mod == "limit" {
				fn.limit = e
			} else {
				fn.offset = e
			}
		case "reversed":
			fn.reversed = true
		default:
			return nil, atOffset(p.peek().pos, "unknown for modifier %q", mod)
		}
	}
	if _, err := p.expect(tTagClose); err != nil {
		return nil, err
	}

	body, stop, err := p.parseBlock(forStops)
	if err != nil {
		return nil, err
	}
	fn.body = body
	if stop == "else" {
		if _, err := p.consumeTagOpen(); err != nil {
			return nil, err
		}
		if _, err := p.expect(tTagClose); err != nil {
			return nil, err
		}
		fn.elseBody, stop, err = p.parseBlock(map[string]bool{"endfor": true})
		if err != nil {
			return nil, err
		}
	}
	if stop != "endfor" {
		return nil, atOffset(open, "{%% for %%} is never closed with {%% endfor %%}")
	}
	if _, err := p.consumeTagOpen(); err != nil {
		return nil, err
	}
	if _, err := p.expect(tTagClose); err != nil {
		return nil, err
	}
	return fn, nil
}

var caseStops = map[string]bool{"when": true, "else": true, "endcase": true}

// parseCase — {% case subject %} {% when v1, v2 %}...{% else %}...{% endcase %}.
// Text between case and the first when is DISCARDED (only whitespace belongs there).
func (p *parser) parseCase() (node, error) {
	open := p.openPos
	subject, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tTagClose); err != nil {
		return nil, err
	}
	_, stop, err := p.parseBlock(caseStops)
	if err != nil {
		return nil, err
	}
	out := caseNode{subject: subject}
	for stop == "when" {
		if _, err := p.consumeTagOpen(); err != nil {
			return nil, err
		}
		w := caseWhen{}
		for {
			// comparison level, NOT parseExpr: `{% when 'a' or 'b' %}` must read
			// `or` as a value separator, not fold into one boolean expression.
			v, err := p.parseComparison()
			if err != nil {
				return nil, err
			}
			w.values = append(w.values, v)
			if p.peek().typ == tComma {
				p.next()
				continue
			}
			// `or`-separated values ({% when 'a' or 'b' %}) as well as commas.
			if p.peek().typ == tIdent && p.peek().val == "or" {
				p.next()
				continue
			}
			break
		}
		if _, err := p.expect(tTagClose); err != nil {
			return nil, err
		}
		w.body, stop, err = p.parseBlock(caseStops)
		if err != nil {
			return nil, err
		}
		out.whens = append(out.whens, w)
	}
	if stop == "else" {
		if _, err := p.consumeTagOpen(); err != nil {
			return nil, err
		}
		if _, err := p.expect(tTagClose); err != nil {
			return nil, err
		}
		out.elseBody, stop, err = p.parseBlock(map[string]bool{"endcase": true})
		if err != nil {
			return nil, err
		}
	}
	if stop != "endcase" {
		return nil, atOffset(open, "{%% case %%} is never closed with {%% endcase %%}")
	}
	if _, err := p.consumeTagOpen(); err != nil {
		return nil, err
	}
	if _, err := p.expect(tTagClose); err != nil {
		return nil, err
	}
	if len(out.whens) == 0 {
		return nil, fmt.Errorf("{%% case %%} needs at least one {%% when %%}")
	}
	return out, nil
}

func (p *parser) parseCapture() (node, error) {
	open := p.openPos
	name, err := p.expect(tIdent)
	if err != nil {
		return nil, err
	}
	if p.reservedBinding(name.val) {
		return nil, atOffset(name.pos, "capture name %q is reserved", name.val)
	}
	if _, err := p.expect(tTagClose); err != nil {
		return nil, err
	}
	body, stop, err := p.parseBlock(map[string]bool{"endcapture": true})
	if err != nil {
		return nil, err
	}
	if stop != "endcapture" {
		return nil, atOffset(open, "{%% capture %%} is never closed with {%% endcapture %%}")
	}
	if _, err := p.consumeTagOpen(); err != nil {
		return nil, err
	}
	if _, err := p.expect(tTagClose); err != nil {
		return nil, err
	}
	return captureNode{name: name.val, body: body}, nil
}

func (p *parser) parseCycle() (node, error) {
	n := cycleNode{id: p.cycleSeq}
	p.cycleSeq++
	for {
		v, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		n.values = append(n.values, v)
		if p.peek().typ == tComma {
			p.next()
			continue
		}
		break
	}
	if _, err := p.expect(tTagClose); err != nil {
		return nil, err
	}
	if len(n.values) == 0 {
		return nil, fmt.Errorf("{%% cycle %%} needs at least one value")
	}
	return n, nil
}

func (p *parser) parseAssign() (node, error) {
	name, err := p.expect(tIdent)
	if err != nil {
		return nil, err
	}
	if p.reservedBinding(name.val) {
		return nil, atOffset(name.pos, "assignment name %q is reserved", name.val)
	}
	if _, err := p.expect(tAssign); err != nil {
		return nil, err
	}
	expr, err := p.parseExpr() // includes a trailing filter pipeline
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tTagClose); err != nil {
		return nil, err
	}
	return assignNode{name: name.val, expr: expr}, nil
}

// Only names bound by the evaluator are reserved for assignments and loops.
// Other application names may legitimately be locals (for example, block).
// reservedBinding reports a name templates cannot bind: settings, forloop
// and the host's CompileOptions.Reserved.
func (p *parser) reservedBinding(name string) bool {
	return name == "settings" || name == "forloop" || p.reserved[name]
}

// parseHostTag parses {% name [expression] %} for a host-declared tag.
func (p *parser) parseHostTag(name string, def TagDefinition) (node, error) {
	pos := p.openPos
	var arg expression
	if p.peek().typ != tTagClose {
		if def.Argument == TagNoArgument {
			return nil, atOffset(p.peek().pos, "{%% %s %%} takes no argument", name)
		}
		var err error
		if arg, err = p.parseExpr(); err != nil {
			return nil, err
		}
	} else if def.Argument == TagRequiredArgument {
		return nil, atOffset(p.peek().pos, "{%% %s %%} requires an argument", name)
	}
	if _, err := p.expect(tTagClose); err != nil {
		return nil, err
	}
	return hostTagNode{name: name, pos: pos, arg: arg}, nil
}

func (p *parser) parseRender() (node, error) {
	comp, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	// Render targets must be literal strings — the static render-target closure (data needs across the
	// render tree) is computed at compile time, so a dynamic target is rejected HERE, not at render.
	if _, ok := literalString(comp); !ok {
		return nil, atOffset(p.peek().pos, "{%% render %%} requires a literal template name (dynamic targets are not allowed)")
	}
	args := map[string]expression{}
	for p.peek().typ == tComma {
		p.next()
		key, err := p.expect(tIdent)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tColon); err != nil {
			return nil, err
		}
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, exists := args[key.val]; exists {
			return nil, atOffset(key.pos, "duplicate render argument %q", key.val)
		}
		if p.reservedBinding(key.val) {
			return nil, atOffset(key.pos, "render argument %q is reserved", key.val)
		}
		args[key.val] = val
	}
	if _, err := p.expect(tTagClose); err != nil {
		return nil, err
	}
	return renderNode{target: comp, args: args}, nil
}

// parseForm parses {% form "action_kind" [, attr: expr ...] %} body {% endform %}. The kind is
// LITERAL-ONLY and validated against the host form profile at COMPILE time — an unknown action
// fails review, never a live render. The tag emits the <form> element with the action path and
// the session CSRF token; the author writes only the inputs that matter.
func (p *parser) parseForm() (node, error) {
	open := p.openPos
	kindExpr, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	kind, ok := literalString(kindExpr)
	if !ok {
		return nil, atOffset(p.peek().pos, "{%% form %%} requires a literal action kind (dynamic kinds are not allowed)")
	}
	definition, known := p.forms[kind]
	if !known {
		return nil, atOffset(p.peek().pos, "{%% form %%}: unknown action kind %q", kind)
	}
	if err := definition.validate(); err != nil {
		return nil, fmt.Errorf("form %q: %w", kind, err)
	}
	attrs := map[string]expression{}
	for p.peek().typ == tComma {
		p.next()
		key, err := p.expect(tIdent)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tColon); err != nil {
			return nil, err
		}
		val, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		name := strings.ToLower(key.val)
		if name == "action" || name == "method" || name == "enctype" || name == strings.ToLower(definition.ActionAttribute) {
			return nil, atOffset(key.pos, "form attribute %q is controlled by the host", key.val)
		}
		if !attributeName.MatchString(key.val) {
			return nil, atOffset(key.pos, "invalid form attribute %q", key.val)
		}
		if _, exists := attrs[name]; exists {
			return nil, atOffset(key.pos, "duplicate form attribute %q", key.val)
		}
		attrs[name] = val
	}
	if _, err := p.expect(tTagClose); err != nil {
		return nil, err
	}
	body, stop, err := p.parseBlock(formStops)
	if err != nil {
		return nil, err
	}
	if stop != "endform" {
		return nil, atOffset(open, "{%% form %%} is never closed with {%% endform %%}")
	}
	if _, err := p.consumeTagOpen(); err != nil {
		return nil, err
	}
	if _, err := p.expect(tTagClose); err != nil {
		return nil, err
	}
	return formNode{kind: kind, definition: definition, attrs: attrs, body: body}, nil
}

// --- expression grammar: filter-pipeline over (or < and < comparison < primary) ---

// parseExpr is the full expression. Filters bind to OPERANDS (see parseOperand), so they work
// everywhere an expression appears (output, assign, if/unless conditions, for limit/offset, render args)
// and `s | upcase == 'HI'` means `(s | upcase) == 'HI'`. NOTE: a filter only ever applies to the operand
// immediately to its left — there is NO way to filter a whole comparison result (`a == b | f` filters b,
// then compares; it does NOT filter `(a == b)`). The trailing parseFilters here only fires when the
// expression is a single operand with no comparison (e.g. `{{ settings.x | upcase }}`).
func (p *parser) parseExpr() (expression, error) {
	if err := p.enter(); err != nil {
		return nil, err
	}
	defer p.leave()
	e, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	return p.parseFilters(e)
}

func (p *parser) parseOr() (expression, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for count := 0; p.peek().typ == tIdent && p.peek().val == "or"; count++ {
		if count >= p.maxNesting {
			return nil, atOffset(p.peek().pos, "compile budget exceeded: boolean expression too deep")
		}
		p.next()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = binaryExpr{op: "or", left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseAnd() (expression, error) {
	left, err := p.parseComparison()
	if err != nil {
		return nil, err
	}
	for count := 0; p.peek().typ == tIdent && p.peek().val == "and"; count++ {
		if count >= p.maxNesting {
			return nil, atOffset(p.peek().pos, "compile budget exceeded: boolean expression too deep")
		}
		p.next()
		right, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		left = binaryExpr{op: "and", left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseComparison() (expression, error) {
	left, err := p.parseOperand()
	if err != nil {
		return nil, err
	}
	var op string
	switch p.peek().typ {
	case tEq:
		op = "=="
	case tNeq:
		op = "!="
	case tGt:
		op = ">"
	case tLt:
		op = "<"
	case tGte:
		op = ">="
	case tLte:
		op = "<="
	case tIdent:
		if p.peek().val == "contains" {
			op = "contains"
		}
	}
	if op == "" {
		return left, nil
	}
	opPos := p.peek().pos
	p.next()
	right, err := p.parseOperand()
	if err != nil {
		return nil, err
	}
	return binaryExpr{op: op, left: left, right: right, pos: opPos}, nil
}

// parseOperand is a primary value optionally followed by a filter pipeline. This is the unit filters
// bind to inside comparisons/and/or, so `s | upcase == 'HI'` filters s before comparing.
func (p *parser) parseOperand() (expression, error) {
	prim, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	return p.parseFilters(prim)
}

func (p *parser) parsePrimary() (expression, error) {
	t := p.peek()
	switch t.typ {
	case tString:
		p.next()
		return literalExpr{val: t.val}, nil
	case tNumber:
		p.next()
		if !strings.ContainsAny(t.val, ".eE") {
			if n, err := strconv.ParseInt(t.val, 10, 64); err == nil {
				return literalExpr{val: n}, nil
			}
		}
		f, err := strconv.ParseFloat(t.val, 64)
		if err != nil {
			return nil, atOffset(t.pos, "bad number %q", t.val)
		}
		return literalExpr{val: f}, nil
	case tLParen:
		return p.parseRange()
	case tIdent:
		switch t.val {
		case "true":
			p.next()
			return literalExpr{val: true}, nil
		case "false":
			p.next()
			return literalExpr{val: false}, nil
		case "nil", "null":
			p.next()
			return literalExpr{val: nil}, nil
		case "empty", "blank":
			p.next()
			return literalExpr{val: emptyMarker{}}, nil
		}
		return p.parseVarPath()
	default:
		return nil, atOffset(t.pos, "unexpected %s in expression", describeToken(t))
	}
}

// parseRange parses (from..to); each bound is a number or a variable path.
func (p *parser) parseRange() (expression, error) {
	open := p.next()
	bound := func() (expression, error) {
		switch p.peek().typ {
		case tNumber, tIdent:
			return p.parsePrimary()
		}
		t := p.peek()
		return nil, atOffset(t.pos, "range bound must be a number or variable")
	}
	from, err := bound()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tRange); err != nil {
		return nil, err
	}
	to, err := bound()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tRParen); err != nil {
		return nil, err
	}
	return rangeExpr{from: from, to: to, pos: open.pos}, nil
}

func (p *parser) parseVarPath() (expression, error) {
	startPos := p.peek().pos
	first, err := p.expect(tIdent)
	if err != nil {
		return nil, err
	}
	segs := []pathSeg{{name: first.val}}
	for {
		switch p.peek().typ {
		case tDot:
			p.next()
			name, err := p.expect(tIdent)
			if err != nil {
				return nil, err
			}
			segs = append(segs, pathSeg{name: name.val})
		case tLBrack:
			p.next()
			idx, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(tRBrack); err != nil {
				return nil, err
			}
			segs = append(segs, pathSeg{idx: idx})
		default:
			return varExpr{segs: segs, pos: startPos}, nil
		}
	}
}

// parseFilters consumes a | filter : arg, arg ... pipeline applied to base.
func (p *parser) parseFilters(base expression) (expression, error) {
	if p.peek().typ != tPipe {
		return base, nil
	}
	var calls []filterCall
	for p.peek().typ == tPipe {
		p.next()
		namePos := p.peek().pos
		name, err := p.expect(tIdent)
		if err != nil {
			return nil, err
		}
		// `raw` does not exist: output is auto-escaped, with no author escape hatch. Reject at compile so
		// a template using it fails compilation. Trusted HTML is the host's job (it hands over SafeHTML values).
		if name.val == "raw" {
			return nil, fmt.Errorf("the 'raw' filter is not available — output is auto-escaped and there is no raw escape hatch (offset %d)", namePos)
		}
		fc := filterCall{name: name.val, pos: namePos}
		if p.peek().typ == tColon {
			p.next()
			for {
				// A named arg is `ident: expr`; a positional arg is a bare expr.
				// Positional args must precede named args, so once a
				// named arg appears no positional arg may follow.
				// Filter arguments are PRIMARIES (literals/vars), not full expressions: a trailing `|`
				// belongs to the OUTER pipeline (`append: "b" | upcase` = `(append "b") | upcase`).
				// Operators are not valid inside filter args.
				if p.peek().typ == tIdent && p.pos+1 < len(p.toks) && p.toks[p.pos+1].typ == tColon {
					key := p.next().val // ident
					p.next()            // colon
					val, err := p.parsePrimary()
					if err != nil {
						return nil, err
					}
					if fc.named == nil {
						fc.named = map[string]expression{}
					}
					if _, dup := fc.named[key]; dup {
						return nil, fmt.Errorf("filter %q: duplicate named argument %q", fc.name, key)
					}
					fc.named[key] = val
				} else {
					if fc.named != nil {
						return nil, fmt.Errorf("filter %q: positional argument after a named argument", fc.name)
					}
					arg, err := p.parsePrimary()
					if err != nil {
						return nil, err
					}
					fc.args = append(fc.args, arg)
				}
				if p.peek().typ != tComma {
					break
				}
				p.next()
			}
		}
		calls = append(calls, fc)
	}
	return filterExpr{base: base, filters: calls}, nil
}

// emptyMarker is the value of the `empty`/`blank` literal — compares equal to any
// empty string/array/map/nil during evaluation.
type emptyMarker struct{}
