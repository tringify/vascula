package vascula

import (
	"errors"
	"fmt"
	"strings"
)

const (
	defaultMaxSteps = 250_000
	defaultMaxBytes = 2 << 20 // 2 MiB per render
)

// Template is a parsed, reusable template. Compile once and cache
// (parsing is the expensive part); Render many times. A cache identity must
// account for both the source and the host's form-profile revision, since
// CompileWithOptions snapshots form definitions into the template.
type Template struct {
	nodes []node
	src   string // retained so runtime errors can report line:col against the original source
}

// Compile parses source into a Template and registers no forms. The host
// controls when to compile and validate. A parse error means the source is
// malformed; parse success does not guarantee render success.
func Compile(src string) (*Template, error) {
	return CompileWithOptions(src, CompileOptions{})
}

// CompileWithOptions parses source and snapshots the host form definitions into
// the compiled template, so later changes to opts.Forms cannot affect it.
func CompileWithOptions(src string, opts CompileOptions) (*Template, error) {
	nodes, err := parseTemplate(src, opts)
	if err != nil {
		return nil, (&Template{src: src}).locate(err)
	}
	return &Template{nodes: markOutputContexts(nodes), src: src}, nil
}

// Child is a resolved child template the evaluator renders for a {% render %}
// tag. The host (which owns its template registry) builds one from a
// template name; the interpreter stays unaware of the registry — it only knows
// how to render a Child against the shared budget. This is the layering seam
// between the registry and the evaluator.
type Child struct {
	// Template is the child's parsed template.
	Template *Template
	// Allow lists the Data roots the child may read. A dotted allowance
	// grants its whole root, not field-level authorization. A {% render %}'d
	// child is a peer with its own contract, not an heir to the caller's
	// namespaces.
	Allow []string
	// Settings are the child's resolved setting values, supplied by the resolver.
	// Named args passed at the render site become local variables, not settings.
	Settings map[string]interface{}
}

// ResolveChild maps a template name to a host-selected Child or an error. The
// interpreter passes no args here: render args are locals, not settings, so they
// don't influence resolution. An error reports a template that cannot be resolved
// (unknown, disallowed kind, cycle policy, …). The host supplies this;
// when nil, {% render %} tags fail at render time.
type ResolveChild func(name string) (Child, error)

// protectedFilter names the safety-critical built-ins a host may NOT override (they define escaping +
// the fallback contract). An override attempt is ignored; the built-in wins.
var protectedFilter = map[string]bool{
	"escape":  true,
	"json":    true,
	"default": true,
}

// maxRenderDepth caps {% render %} nesting so a recursive composition (a→b→a)
// aborts cleanly before exhausting the goroutine stack — the budget bounds total
// work, this bounds depth.
const maxRenderDepth = 32

// Options configures a single render.
type Options struct {
	// Data is host-supplied data by root name. Only roots named in Allow are
	// readable; reading any other name that is not a variable is an
	// UndeclaredNameError.
	Data map[string]interface{}
	// Allow lists the readable Data roots. A dotted entry grants its whole
	// root, not field-level authorization.
	Allow []string
	// Settings are this template's parameters, readable as settings without
	// an allowance. A child has its own (see Child).
	Settings map[string]interface{}
	// Globals are readable by name everywhere without an allowance, in this
	// template and every child. Reserve their names at compilation
	// (CompileOptions.Reserved) so templates cannot shadow them.
	Globals map[string]interface{}
	// Filters are trusted host extensions layered over the built-ins. A host
	// filter wins on a name collision, EXCEPT the safety-critical built-ins
	// escape, json and default, which a host cannot override.
	Filters map[string]FilterFunc
	// Resolve resolves child templates for {% render %}. nil makes render tags
	// fail at render time. A child shares this render's budget.
	Resolve ResolveChild
	// Tags implements the host tags declared in CompileOptions.Tags. Their
	// markup is trusted and not sanitized; the host enforces its own limits.
	// A declared tag with no implementation fails when it renders.
	Tags map[string]TagFunc
	// MaxSteps / MaxBytes override the render budget. A nonpositive value uses
	// the default.
	MaxSteps int
	MaxBytes int
	// MaxValueBytes and MaxValueNodes bound each value processed by rendering
	// or filters, including unprinted intermediate values. Nonpositive values
	// use defaults of 2 MiB and 50,000 value nodes. This is not a process quota.
	// Sequence loops bound entry count; nested fields are bounded when consumed.
	MaxValueBytes int
	MaxValueNodes int
}

// Render evaluates the template against opts and returns the produced HTML.
// Ordinary interpolation values are HTML-escaped. Literal template markup,
// SafeHTML values and host-generated markup are trusted and emitted as-is; this
// is not a JavaScript, CSS or URL sanitizer. There is no author-facing escape
// hatch (no `raw` filter, no {% raw %} block).
func (t *Template) Render(opts Options) (string, error) {
	if t == nil {
		return "", fmt.Errorf("cannot render a nil template")
	}
	allowed := make(map[string]bool, len(opts.Allow))
	for _, a := range opts.Allow {
		// A dotted need ("cart.item_count") grants its ROOT namespace — the dot
		// suffix is a host enrichment tier hint (cheap vs rich), not a narrower
		// access scope; the host only builds the tier the needs asked for.
		if i := strings.IndexByte(a, '.'); i >= 0 {
			a = a[:i]
		}
		allowed[a] = true
	}

	work := &budget{
		maxSteps:      orDefault(opts.MaxSteps, defaultMaxSteps),
		maxOut:        orDefault(opts.MaxBytes, defaultMaxBytes),
		maxValueBytes: orDefault(opts.MaxValueBytes, defaultMaxValueBytes),
		maxValueNodes: orDefault(opts.MaxValueNodes, defaultMaxValueNodes),
	}
	filters := boundedBuiltinFilters(work)
	hostFilters := make(map[string]bool, len(opts.Filters))
	for name, fn := range opts.Filters {
		// Host filters extend/override built-ins, EXCEPT the safety-critical ones: escape/json/default
		// define the escaping + fallback contract and must not be silently replaced by a host. Ignore an
		// attempt to override them (the built-in wins) so the security contract is stable.
		if protectedFilter[name] {
			continue
		}
		filters[name] = fn
		hostFilters[name] = true
	}

	root := topFrame(opts.Settings, opts.Globals)

	var out strings.Builder
	e := &evalCtx{
		root:        orEmptyMap(opts.Data),
		allowed:     allowed,
		locals:      root,
		globals:     opts.Globals,
		filters:     filters,
		hostFilters: hostFilters,
		out:         &out,
		resolve:     opts.Resolve,
		tags:        opts.Tags,
		budget:      work,
	}
	if err := e.renderNodes(t.nodes); err != nil {
		return "", t.locate(err)
	}
	return out.String(), nil
}

// locate prefixes a positioned runtime error with "line L, col C" against the template source. Errors
// without a position (budget, render-depth) pass through unchanged.
func (t *Template) locate(err error) error {
	var located *locatedError
	if errors.As(err, &located) {
		return err
	}
	var pe *posError
	if !asPosError(err, &pe) {
		return err
	}
	line, col := lineCol(t.src, pe.pos)
	return &locatedError{err: err, line: line, col: col}
}

type locatedError struct {
	err       error
	line, col int
}

func (e *locatedError) Error() string {
	return fmt.Sprintf("%s (line %d, col %d)", e.err, e.line, e.col)
}
func (e *locatedError) Unwrap() error { return e.err }

func orEmptyMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return map[string]interface{}{}
	}
	return m
}

func orDefault(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}
