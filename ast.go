package vascula

// AST for the template dialect. A parsed template is a []node; control-flow nodes
// hold child []node bodies. Expressions form their own small tree.

type node interface{ nodeMarker() }

type textNode struct{ text string }

// outputNode is {{ expr }}; expr may be a filterExpr. inTag is set at compile
// time when the tag sits inside an HTML tag (see html_context.go).
type outputNode struct {
	expr       expression
	inTag      bool
	inHTMLText bool
}

type ifBranch struct {
	cond expression
	body []node
}

// ifNode covers {% if %}/{% elsif %}/{% else %} and {% unless %} (negated single
// branch). branches are evaluated in order; the first truthy one renders.
type ifNode struct {
	branches []ifBranch
	elseBody []node
}

type forNode struct {
	varName  string
	coll     expression
	limit    expression // optional
	offset   expression // optional
	reversed bool
	body     []node
	elseBody []node // {% else %} renders when the collection is empty
}

type assignNode struct {
	name string
	expr expression
}

// renderNode renders a child template in its OWN scope (no leakage from the
// caller), passing named args as locals.
// hostTagNode is a host-declared tag; arg is nil when none was written.
type hostTagNode struct {
	name string
	pos  int
	arg  expression
}

type formNode struct {
	definition FormDefinition
	kind       string                // host-defined literal kind, validated at compile
	attrs      map[string]expression // literal-ish HTML attributes (class: "x"), attribute-escaped at render
	body       []node
}

type renderNode struct {
	target expression
	args   map[string]expression
}

// (slot is rejected at COMPILE time — see parser.go — so there is no slotNode in the AST. It returns
// when composition children are wired.)

// caseNode is {% case %}/{% when %}/{% else %}. Each when may list several
// values (comma or `or` separated) — the first branch containing a value equal
// to the subject renders.
type caseNode struct {
	subject  expression
	whens    []caseWhen
	elseBody []node
}

type caseWhen struct {
	values []expression
	body   []node
}

// captureNode renders its body into a string local ({% capture x %}...{% endcapture %}).
type captureNode struct {
	name string
	body []node
}

// breakNode / continueNode are loop control ({% break %} / {% continue %}) —
// evaluated as sentinel errors the innermost for-loop absorbs.
type breakNode struct{}
type continueNode struct{}

// cycleNode rotates through its values on successive renders ({% cycle 'a','b' %}).
// id is assigned at parse time; rotation state lives on the evaluator per render.
type cycleNode struct {
	id     int
	values []expression
}

func (textNode) nodeMarker()     {}
func (outputNode) nodeMarker()   {}
func (ifNode) nodeMarker()       {}
func (forNode) nodeMarker()      {}
func (assignNode) nodeMarker()   {}
func (renderNode) nodeMarker()   {}
func (caseNode) nodeMarker()     {}
func (captureNode) nodeMarker()  {}
func (breakNode) nodeMarker()    {}
func (continueNode) nodeMarker() {}
func (cycleNode) nodeMarker()    {}
func (formNode) nodeMarker()     {}
func (hostTagNode) nodeMarker()  {}

// --- expressions ---

type expression interface{ exprMarker() }

// literalExpr holds a string, int64 (a number without a fraction), float64,
// bool, or nil.
type literalExpr struct{ val interface{} }

// pathSeg is one step of a variable path: a field name, or a bracket index whose
// value is itself an expression (product.images[i], cart["key"]).
type pathSeg struct {
	name string
	idx  expression // nil for .name access
}

// varExpr carries the byte offset of its first segment, so an undeclared-access error points at the var.
type varExpr struct {
	segs []pathSeg
	pos  int
}

// binaryExpr is a comparison or boolean op: == != > < >= <= and or contains. pos is the operator offset.
type binaryExpr struct {
	op          string
	left, right expression
	pos         int
}

// notExpr is logical negation of a condition's TRUTHINESS — used by {% unless %}.
// (unless cond renders when cond is falsy; this is !truthy(cond), NOT cond == false.)
type notExpr struct{ inner expression }

// filterCall is one stage of a filter pipeline. Args may be positional (truncate:
// 20) and/or named (image_url: width: 200); the grammar requires positional args
// to precede named ones. named is nil when the call uses no named args. pos = the filter-name offset.
type filterCall struct {
	name  string
	args  []expression
	named map[string]expression
	pos   int
}

// filterExpr applies a pipeline of filters to a base value.
type filterExpr struct {
	base    expression
	filters []filterCall
}

func (literalExpr) exprMarker() {}
func (varExpr) exprMarker()     {}
func (binaryExpr) exprMarker()  {}
func (notExpr) exprMarker()     {}
func (filterExpr) exprMarker()  {}
func (rangeExpr) exprMarker()   {}

// rangeExpr is (from..to): the integers from through to, inclusive. Each bound
// is a number or a variable path; pos is the opening parenthesis.
type rangeExpr struct {
	from, to expression
	pos      int
}
