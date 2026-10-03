package vascula

import (
	"fmt"
	"html"
	"sort"
	"strings"
)

// posError carries a byte offset into the template source so the Render boundary can render a
// human-readable "line L, col C" prefix. Runtime (eval-time) errors wrap themselves with a position via
// failAt; parse errors already carry offsets in their text. A nil-pos (0 with no source) degrades to the
// bare message.
type posError struct {
	pos int
	err error
}

func (e *posError) Error() string { return e.err.Error() }
func (e *posError) Unwrap() error { return e.err }

// failAt wraps err with a source position, unless it already has one (the innermost position wins, so a
// nested expression's error keeps pointing at the real offender).
func failAt(pos int, err error) error {
	if err == nil {
		return nil
	}
	var pe *posError
	if asPosError(err, &pe) {
		return err
	}
	return &posError{pos: pos, err: err}
}

// atOffset is a compilation error at a byte offset; Compile reports it as a
// line and column.
func atOffset(pos int, format string, args ...interface{}) error {
	return &posError{pos: pos, err: fmt.Errorf(format, args...)}
}

func asPosError(err error, target **posError) bool {
	for err != nil {
		if pe, ok := err.(*posError); ok {
			*target = pe
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

// lineCol converts a byte offset into a 1-based line and a 1-based column
// counted in characters, as an editor shows them.
func lineCol(src string, pos int) (int, int) {
	if pos < 0 {
		pos = 0
	}
	if pos > len(src) {
		pos = len(src)
	}
	line, col := 1, 1
	for _, r := range src[:pos] {
		if r == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}

// scope is a frame of local variables. A render has one top-level frame (the
// template's own variables, settings and render arguments); each loop adds a
// frame holding only its loop variable and forloop. Branches add none.
//
// assign and capture update the nearest existing binding, otherwise they bind
// in the top-level frame, so a value set inside if, for, case or capture is
// still there afterwards (as in Liquid). Loop variables stay local to their
// loop. A {% render %}ed child has its own top-level frame and never sees the
// caller's variables.
type scope struct {
	vars   map[string]interface{} // allocated on first binding
	parent *scope
	// A loop frame holds its variable and forloop in fields, not the map.
	loop     *loopInfo
	loopName string
	loopItem interface{}
}

func newScope(parent *scope) *scope {
	return &scope{parent: parent}
}

func (s *scope) get(name string) (interface{}, bool) {
	for c := s; c != nil; c = c.parent {
		if c.loop != nil {
			if name == c.loopName {
				return c.loopItem, true
			}
			if name == "forloop" {
				return c.loop, true
			}
		}
		if v, ok := c.vars[name]; ok {
			return v, true
		}
	}
	return nil, false
}

func (s *scope) set(name string, v interface{}) {
	top := s
	for c := s; c != nil; c = c.parent {
		if c.loop != nil && name == c.loopName {
			c.loopItem = v
			return
		}
		if _, ok := c.vars[name]; ok {
			c.vars[name] = v
			return
		}
		top = c
	}
	top.setLocal(name, v)
}

func (s *scope) setLocal(name string, v interface{}) {
	if s.vars == nil {
		s.vars = make(map[string]interface{}, 2)
	}
	s.vars[name] = v
}

// budget bounds the work a single render may do — the backstop against a
// pathological template (deep loops, huge output). Exceeding it aborts the render.
type budget struct {
	steps         int
	maxSteps      int
	outBytes      int
	maxOut        int
	maxValueBytes int
	maxValueNodes int
	// validated remembers maps and slices already checked by value during this
	// render, so a collection passed along a filter chain is walked once.
	validated map[valueIdentity]validatedValue
}

func (b *budget) step() error {
	return b.spend(1)
}

func (b *budget) spend(n int) error {
	if n < 0 || n > b.maxSteps-b.steps {
		return fmt.Errorf("render budget exceeded: too many steps (>%d)", b.maxSteps)
	}
	b.steps += n
	return nil
}

func (b *budget) wrote(n int) error {
	if n < 0 || n > b.maxOut-b.outBytes {
		return fmt.Errorf("render budget exceeded: output too large (>%d bytes)", b.maxOut)
	}
	b.outBytes += n
	return nil
}

// evalCtx is one render's mutable state.
type evalCtx struct {
 finalOut *strings.Builder
 observeOutput func(OutputSpan)

	root        map[string]interface{} // host data (access-gated by allowed)
	allowed     map[string]bool        // allowed data roots
	locals      *scope                 // local vars: settings, globals, assigns, loop vars, args
	globals     map[string]interface{} // bound in EVERY top-level frame, children included
	filters     map[string]FilterFunc
	hostFilters map[string]bool // names served by host filters, which receive JSON-shaped numbers
	out         *strings.Builder
	budget      *budget
	resolve     ResolveChild       // child resolver for {% render %} (may be nil)
	tags        map[string]TagFunc // host tag implementations
	depth       int                // current {% render %} nesting depth
	exprDepth   int
	cycleState  map[int]int // {% cycle %} rotation per tag occurrence (render-scoped)
}

func (e *evalCtx) renderNodes(nodes []node) error {
	for _, n := range nodes {
		if err := e.renderNode(n); err != nil {
			return err
		}
	}
	return nil
}

func (e *evalCtx) renderNode(n node) error {
	if err := e.budget.step(); err != nil {
		return err
	}
	switch x := n.(type) {
	case textNode:
		return e.write(safeString(x.text)) // literal markup is already HTML
	case outputNode:
		v, err := e.evalExpr(x.expr)
		if err != nil {
			return err
		}
		start:=e.out.Len()
        if err:=e.writeIn(v,x.inTag);err!=nil{return err}
        if e.observeOutput!=nil && x.inHTMLText && e.out==e.finalOut {
          if direct,ok:=x.expr.(varExpr);ok {
            path:=make([]string,0,len(direct.segs));for _,seg:=range direct.segs {if seg.idx!=nil {path=nil;break};path=append(path,seg.name)}
            if len(path)>1 && e.out.Len()>start {
              root,err:=e.resolveVar(varExpr{segs:direct.segs[:1],pos:direct.pos});if err!=nil{return err}
              e.observeOutput(OutputSpan{Path:path,RootValue:root,Start:start,End:e.out.Len()})
            }
          }
        }
        return nil
	case ifNode:
		return e.renderIf(x)
	case forNode:
		return e.renderFor(x)
	case assignNode:
		v, err := e.evalExpr(x.expr)
		if err != nil {
			return err
		}
		e.locals.set(x.name, v)
		return nil
	case renderNode:
		return e.renderChild(x)
	case formNode:
		return e.renderForm(x)
	case hostTagNode:
		return e.renderHostTag(x)
	case caseNode:
		return e.renderCase(x)
	case captureNode:
		var sb strings.Builder
		prev := e.out
		e.out = &sb
		err := e.renderInScope(x.body)
		e.out = prev
		if err != nil {
			return err
		}
		e.locals.set(x.name, sb.String())
		return nil
	case breakNode:
		return errBreak
	case continueNode:
		return errContinue
	case cycleNode:
		if e.cycleState == nil {
			e.cycleState = map[int]int{}
		}
		idx := e.cycleState[x.id] % len(x.values)
		e.cycleState[x.id]++
		v, err := e.evalExpr(x.values[idx])
		if err != nil {
			return err
		}
		return e.write(v)
	default:
		return fmt.Errorf("unrenderable node %T", n)
	}
}

// renderCase evaluates {% case %}: the first when containing a value equal to
// the subject renders; none → else body.
func (e *evalCtx) renderCase(n caseNode) error {
	subject, err := e.evalExpr(n.subject)
	if err != nil {
		return err
	}
	if err := e.budget.value(subject); err != nil {
		return err
	}
	for _, w := range n.whens {
		for _, vx := range w.values {
			v, err := e.evalExpr(vx)
			if err != nil {
				return err
			}
			if err := e.budget.value(v); err != nil {
				return err
			}
			if equalValues(subject, v) {
				return e.renderInScope(w.body)
			}
		}
	}
	return e.renderInScope(n.elseBody)
}

// renderChild evaluates a {% render %} of a child template. The child renders in
// its OWN scope (no caller-local leakage): named args become
// child LOCALS (`{% render 'c', x: 1 %}` → `{{ x }}`), the child keeps its
// OWN settings, reads only its OWN allowed data roots (a peer contract, not
// inherited), and shares this render's output sink and budget so the whole
// composition tree is bounded as one unit.
func (e *evalCtx) renderChild(n renderNode) error {
	if e.resolve == nil {
		return fmt.Errorf("{%% render %%} unavailable: no child resolver wired")
	}
	if e.depth >= maxRenderDepth {
		return fmt.Errorf("{%% render %%} nesting too deep (>%d): recursive composition?", maxRenderDepth)
	}

	// Literal targets let hosts inspect the complete composition graph before rendering.
	name, ok := literalString(n.target)
	if !ok {
		return fmt.Errorf("{%% render %%} requires a literal template name (dynamic targets are not allowed)")
	}

	child, err := e.resolve(name)
	if err != nil {
		return fmt.Errorf("{%% render %%} %q: %w", name, err)
	}
	if child.Template == nil {
		return fmt.Errorf("{%% render %%} %q: resolver returned a nil template", name)
	}

	allowed := make(map[string]bool, len(child.Allow))
	for _, a := range child.Allow {
		if i := strings.IndexByte(a, '.'); i >= 0 {
			a = a[:i] // dotted need grants the root (tier hint, not scope)
		}
		allowed[a] = true
	}
	// A child has its own top-level frame: its own settings, the same globals,
	// and the named arguments as variables (reserved names fail compilation).
	childRoot := topFrame(child.Settings, e.globals)
	for k, ex := range n.args {
		v, err := e.evalExpr(ex)
		if err != nil {
			return err
		}
		childRoot.setLocal(k, v)
	}

	ce := &evalCtx{
		root:        e.root,
		allowed:     allowed,
		locals:      childRoot,
		globals:     e.globals,
		filters:     e.filters,
		hostFilters: e.hostFilters,
		tags:        e.tags,
		out:         e.out,
		budget:      e.budget,
		resolve:     e.resolve,
		depth:       e.depth + 1,
	}
	if err := ce.renderNodes(child.Template.nodes); err != nil {
		return fmt.Errorf("{%% render %%} %q: %w", name, child.Template.locate(err))
	}
	return nil
}

// literalString reports whether ex is a bare string literal (the only render
// target the language accepts) and returns its value.
func literalString(ex expression) (string, bool) {
	lit, ok := ex.(literalExpr)
	if !ok {
		return "", false
	}
	s, ok := lit.val.(string)
	return s, ok
}

func (e *evalCtx) renderIf(n ifNode) error {
	for _, br := range n.branches {
		v, err := e.evalExpr(br.cond)
		if err != nil {
			return err
		}
		if truthy(v) {
			return e.renderInScope(br.body)
		}
	}
	return e.renderInScope(n.elseBody)
}

// errBreak / errContinue are loop-control sentinels: {% break %}/{% continue %}
// unwind to the INNERMOST for loop, which absorbs them. Outside any loop they
// surface as render errors (a bare {% break %} is an author bug).
var errBreak = fmt.Errorf("{%% break %%} outside a for loop")
var errContinue = fmt.Errorf("{%% continue %%} outside a for loop")

func (e *evalCtx) renderFor(n forNode) error {
	collVal, err := e.evalExpr(n.coll)
	if err != nil {
		return err
	}
	if err := e.budget.loopCollection(collVal); err != nil {
		return err
	}
	items := iterate(collVal)

	if n.offset != nil {
		off, err := e.evalInt(n.offset) // errors (e.g. an undeclared name) must surface, never swallow
		if err != nil {
			return err
		}
		if off > 0 && off <= len(items) {
			items = items[off:]
		} else if off > len(items) {
			items = nil
		}
	}
	if n.limit != nil {
		lim, err := e.evalInt(n.limit)
		if err != nil {
			return err
		}
		if lim >= 0 && lim < len(items) {
			items = items[:lim]
		}
	}
	if n.reversed {
		rev := make([]interface{}, len(items))
		for i, v := range items {
			rev[len(items)-1-i] = v
		}
		items = rev
	}

	if len(items) == 0 {
		return e.renderInScope(n.elseBody)
	}

	parent := e.locals
	outer, _ := parent.get("forloop")
	outerLoop, _ := outer.(*loopInfo)
	loop := &scope{parent: parent, loopName: n.varName}
	e.locals = loop
	defer func() { e.locals = parent }()
	total := len(items)
	for i, item := range items {
		if err := e.budget.step(); err != nil {
			return err
		}
		loop.loopItem = item
		loop.loop = &loopInfo{index: i, length: total, parent: outerLoop}
		if err := e.renderNodes(n.body); err != nil {
			if err == errBreak {
				return nil
			}
			if err == errContinue {
				continue
			}
			return err
		}
	}
	return nil
}

// renderInScope renders a branch or capture body. Branches share the
// enclosing frame (see scope), so this adds no frame and allocates nothing.
func (e *evalCtx) renderInScope(nodes []node) error {
	return e.renderNodes(nodes)
}

func (e *evalCtx) write(v interface{}) error { return e.writeIn(v, false) }

// writeIn writes v; inTag is true for an output tag inside an HTML tag, where
// JSON text is escaped like any other string.
func (e *evalCtx) writeIn(v interface{}, inTag bool) error {
	if err := e.budget.value(v); err != nil {
		return err
	}
	var s string
	switch ss := v.(type) {
	case safeString:
		s = string(ss)
	case jsonString:
		s = string(ss)
		if inTag {
			s = html.EscapeString(s)
		}
	case SafeHTML: // host-vouched trusted HTML — emit as-is, no auto-escape
		s = string(ss)
	default:
		s = html.EscapeString(toString(v))
	}
	if err := e.budget.wrote(len(s)); err != nil {
		return err
	}
	e.out.WriteString(s)
	return nil
}

// --- expression evaluation ---

func (e *evalCtx) evalExpr(ex expression) (interface{}, error) {
	if err := e.budget.step(); err != nil {
		return nil, err
	}
	if e.exprDepth >= defaultMaxExpressionDepth {
		return nil, fmt.Errorf("render budget exceeded: expression too deep")
	}
	e.exprDepth++
	defer func() { e.exprDepth-- }()
	switch x := ex.(type) {
	case literalExpr:
		return x.val, nil
	case varExpr:
		return e.resolveVar(x)
	case binaryExpr:
		return e.evalBinary(x)
	case notExpr:
		v, err := e.evalExpr(x.inner)
		if err != nil {
			return nil, err
		}
		return !truthy(v), nil
	case filterExpr:
		return e.evalFilters(x)
	case rangeExpr:
		return e.evalRange(x)
	default:
		return nil, fmt.Errorf("uninterpretable expression %T", ex)
	}
}

// resolveVar is the access-control boundary. The first path segment must be a
// local OR an allowed data root; anything else is a hard error
// (declare-then-resolve), not a silent empty. A missing sub-path IS nil.
// renderForm emits a POST form using the definition bound at compilation.
// The token is read directly from the host-selected context key; it is not
// exposed to ordinary template expressions unless the host grants that key.
// The host is responsible for token issuance and request verification.
func (e *evalCtx) renderForm(x formNode) error {
	path := x.definition.Path
	csrf, _ := e.root[x.definition.TokenContextKey].(string)
	if csrf == "" {
		return fmt.Errorf("{%% form %%}: host token missing from context key %q", x.definition.TokenContextKey)
	}
	if err := e.budget.value(csrf); err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString(`<form method="post" action="`)
	b.WriteString(html.EscapeString(path))
	b.WriteString(`"`)
	if x.definition.ActionAttribute != "" {
		b.WriteString(" " + x.definition.ActionAttribute + `="`)
		b.WriteString(html.EscapeString(x.kind))
		b.WriteString(`"`)
	}
	if x.definition.Multipart {
		b.WriteString(` enctype="multipart/form-data"`)
	}
	attrNames := make([]string, 0, len(x.attrs))
	for name := range x.attrs {
		attrNames = append(attrNames, name)
	}
	sort.Strings(attrNames)
	for _, name := range attrNames {
		v, err := e.evalExpr(x.attrs[name])
		if err != nil {
			return err
		}
		if err := e.budget.value(v); err != nil {
			return err
		}
		b.WriteString(" ")
		b.WriteString(name)
		b.WriteString(`="`)
		b.WriteString(html.EscapeString(toString(v)))
		b.WriteString(`"`)
	}
	b.WriteString(`><input type="hidden" name="`)
	b.WriteString(html.EscapeString(x.definition.TokenField))
	b.WriteString(`" value="`)
	b.WriteString(html.EscapeString(csrf))
	b.WriteString(`">`)
	if err := e.write(safeString(b.String())); err != nil {
		return err
	}
	for _, n := range x.body {
		if err := e.renderNode(n); err != nil {
			return err
		}
	}
	return e.write(safeString("</form>"))
}

func (e *evalCtx) resolveVar(v varExpr) (interface{}, error) {
	root := v.segs[0].name
	var cur interface{}
	if val, ok := e.locals.get(root); ok {
		cur = val
	} else if e.allowed[root] {
		cur = e.root[root]
	} else {
		return nil, failAt(v.pos, &UndeclaredNameError{Name: root})
	}

	for _, seg := range v.segs[1:] {
		if err := e.budget.step(); err != nil {
			return nil, err
		}
		if cur == nil {
			// A nil chain has no fields — EXCEPT a trailing .size, which is 0 (a missing collection has
			// zero elements), so `{% if X.items.size > 0 %}` is safe when any link in X.items is absent.
			if last := v.segs[len(v.segs)-1]; last.idx == nil && last.name == "size" {
				return float64(0), nil
			}
			return nil, nil
		}
		if seg.idx != nil {
			idx, err := e.evalExpr(seg.idx)
			if err != nil {
				return nil, err
			}
			cur = indexValue(cur, idx)
		} else {
			cur = fieldValue(cur, seg.name)
		}
	}
	return cur, nil
}

func (e *evalCtx) evalBinary(b binaryExpr) (interface{}, error) {
	// Short-circuit boolean ops. They return the OPERAND VALUE, not a bool:
	//   a or b  → a if a is truthy, else b   (so `name or 'Guest'` yields the name or "Guest")
	//   a and b → b if a is truthy, else a   (so a falsy left short-circuits to that left value)
	// Used in an if/unless condition, the returned value is then tested for truthiness as usual.
	if b.op == "and" || b.op == "or" {
		l, err := e.evalExpr(b.left)
		if err != nil {
			return nil, err
		}
		lt := truthy(l)
		if b.op == "or" {
			if lt {
				return l, nil
			}
			return e.evalExpr(b.right)
		}
		// and
		if !lt {
			return l, nil
		}
		return e.evalExpr(b.right)
	}

	l, err := e.evalExpr(b.left)
	if err != nil {
		return nil, err
	}
	r, err := e.evalExpr(b.right)
	if err != nil {
		return nil, err
	}
	if err := e.budget.value(l); err != nil {
		return nil, err
	}
	if err := e.budget.value(r); err != nil {
		return nil, err
	}
	switch b.op {
	case "==":
		return equalValues(l, r), nil
	case "!=":
		return !equalValues(l, r), nil
	case "contains":
		return containsValue(l, r), nil
	case ">", "<", ">=", "<=":
		c, ok := compareValues(l, r)
		if !ok {
			// Cross-type / unorderable comparison is an error (e.g. number > string) — surfaced, never a
			// silent false. == / != stay loose; ordering requires same-kind operands.
			return nil, failAt(b.pos, fmt.Errorf("cannot compare %v and %v with %q", typeName(l), typeName(r), b.op))
		}
		switch b.op {
		case ">":
			return c > 0, nil
		case "<":
			return c < 0, nil
		case ">=":
			return c >= 0, nil
		default:
			return c <= 0, nil
		}
	}
	return nil, fmt.Errorf("unknown operator %q", b.op)
}

func (e *evalCtx) evalFilters(f filterExpr) (interface{}, error) {
	v, err := e.evalExpr(f.base)
	if err != nil {
		return nil, err
	}
	for _, call := range f.filters {
		if err := e.budget.step(); err != nil {
			return nil, err
		}
		fn, ok := e.filters[call.name]
		if !ok || fn == nil {
			return nil, failAt(call.pos, fmt.Errorf("unknown filter %q", call.name))
		}
		fa := FilterArgs{Pos: make([]interface{}, len(call.args))}
		for i, a := range call.args {
			av, err := e.evalExpr(a)
			if err != nil {
				return nil, err
			}
			fa.Pos[i] = av
		}
		if len(call.named) > 0 {
			// A built-in takes only the named arguments it documents. Anything
			// else is a mistake, typically a render argument the filter's
			// argument list swallowed: {% render "x", a: b | plus: 1, c: d %}.
			if !e.hostFilters[call.name] {
				for name := range call.named {
					if !builtinNamedArgs[call.name][name] {
						return nil, failAt(call.pos, fmt.Errorf("filter %q does not take a named argument %q", call.name, name))
					}
				}
			}
			fa.Named = make(map[string]interface{}, len(call.named))
			for k, a := range call.named {
				av, err := e.evalExpr(a)
				if err != nil {
					return nil, err
				}
				fa.Named[k] = av
			}
		}
		check := e.budget.value
		if shallowFilters[call.name] && !e.hostFilters[call.name] {
			check = e.budget.collection
		}
		if err := check(v); err != nil {
			return nil, failAt(call.pos, err)
		}
		for _, arg := range fa.Pos {
			if err := e.budget.value(arg); err != nil {
				return nil, failAt(call.pos, err)
			}
		}
		for _, arg := range fa.Named {
			if err := e.budget.value(arg); err != nil {
				return nil, failAt(call.pos, err)
			}
		}
		if e.hostFilters[call.name] {
			v = hostValue(v)
			for i := range fa.Pos {
				fa.Pos[i] = hostValue(fa.Pos[i])
			}
			for k, arg := range fa.Named {
				fa.Named[k] = hostValue(arg)
			}
		}
		v, err = fn(v, fa)
		if err != nil {
			return nil, failAt(call.pos, fmt.Errorf("filter %q: %w", call.name, err))
		}
		if err := check(v); err != nil {
			return nil, failAt(call.pos, err)
		}
	}
	return v, nil
}

// evalRange builds (from..to) as integers. A reversed range is empty. Its
// length is bounded like any other value.
func (e *evalCtx) evalRange(r rangeExpr) (interface{}, error) {
	from, err := e.evalInt(r.from)
	if err != nil {
		return nil, failAt(r.pos, fmt.Errorf("range: %w", err))
	}
	to, err := e.evalInt(r.to)
	if err != nil {
		return nil, failAt(r.pos, fmt.Errorf("range: %w", err))
	}
	if to < from {
		return []interface{}{}, nil
	}
	if int64(to)-int64(from) >= int64(e.budget.maxValueNodes) {
		return nil, failAt(r.pos, fmt.Errorf("render budget exceeded: range too large"))
	}
	n := to - from + 1
	if err := e.budget.spend(n); err != nil {
		return nil, err
	}
	out := make([]interface{}, n)
	for i := range out {
		out[i] = int64(from + i)
	}
	return out, nil
}

func (e *evalCtx) evalInt(ex expression) (int, error) {
	v, err := e.evalExpr(ex)
	if err != nil {
		return 0, err
	}
	f, ok := toFloat(v)
	if !ok {
		return 0, fmt.Errorf("expected a number")
	}
	return intFromFloat(f)
}

// renderHostTag runs a host tag and writes its trusted markup.
func (e *evalCtx) renderHostTag(x hostTagNode) error {
	fn := e.tags[x.name]
	if fn == nil {
		return failAt(x.pos, fmt.Errorf("{%% %s %%}: the host does not implement this tag", x.name))
	}
	call := TagCall{Name: x.name, lookup: e.lookup}
	if x.arg != nil {
		v, err := e.evalExpr(x.arg)
		if err != nil {
			return err
		}
		if err := e.budget.value(v); err != nil {
			return failAt(x.pos, err)
		}
		call.Arg, call.HasArg = v, true
	}
	out, err := fn(call)
	if err != nil {
		return failAt(x.pos, fmt.Errorf("{%% %s %%}: %w", x.name, err))
	}
	return e.write(out)
}

// lookup reads what a template could read by name at this point.
func (e *evalCtx) lookup(name string) (interface{}, bool) {
	if v, ok := e.locals.get(name); ok {
		return v, true
	}
	if e.allowed[name] {
		return e.root[name], true
	}
	return nil, false
}

// topFrame is a render's top-level frame: settings and the globals.
func topFrame(settings, globals map[string]interface{}) *scope {
	frame := newScope(nil)
	frame.setLocal("settings", orEmptyMap(settings))
	for name, value := range globals {
		frame.setLocal(name, value)
	}
	return frame
}
