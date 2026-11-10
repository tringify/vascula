package vascula

import "sort"

// NameUse is a name a template reads but never defines itself, with the line
// and character column of its first use.
type NameUse struct {
	Name      string
	Line, Col int
}

// ExternalNames returns, in source order, every root name the template reads
// without defining it: names the host must provide through Data (and allow), or
// a caller through {% render %} arguments. A host compares them with what it
// will supply and refuses a template that reads anything else at publish time,
// instead of failing on the first render that reaches the read.
//
// Names bound by assign or capture anywhere in the template, loop variables
// inside their loop, forloop inside a loop, settings, and the names passed as
// globals are defined. Every branch is visited, including ones that may not
// execute. Reading an assigned name before its assignment runs is not
// detected; that remains a render error.
func (t *Template) ExternalNames(globals ...string) []NameUse {
	bound := map[string]bool{}
	for _, name := range globals {
		bound[name] = true
	}
	collectBoundNames(t.nodes, bound)
	w := nameWalker{src: t.src, bound: bound, seen: map[string]bool{}}
	w.nodes(t.nodes)
	return w.out
}

// ExternalPaths returns, in source order, every distinct path the template
// reads from names it does not define, written as dotted names: "order.number",
// or "order.items" when a loop goes over that collection. A read through the
// variable of a loop over an external collection is reported through the
// collection followed by "[]": {% for item in order.items %}{{ item.title }}
// reads "order.items[].title". A path ends before its first bracketed segment
// (items[0], map[key]); the expression inside the brackets is reported on its
// own. A host compares the paths with the data it supplies and refuses a
// template that reads a field it does not have.
//
// Definitions follow ExternalNames. A loop over a defined name, a range or a
// literal binds a variable whose reads are not reported.
func (t *Template) ExternalPaths(globals ...string) []NameUse {
	bound := map[string]bool{}
	for _, name := range globals {
		bound[name] = true
	}
	collectBoundNames(t.nodes, bound)
	w := nameWalker{src: t.src, bound: bound, seen: map[string]bool{}, paths: true}
	w.nodes(t.nodes)
	return w.out
}

func collectBoundNames(nodes []node, bound map[string]bool) {
	for _, n := range nodes {
		switch v := n.(type) {
		case assignNode:
			bound[v.name] = true
		case captureNode:
			bound[v.name] = true
			collectBoundNames(v.body, bound)
		case ifNode:
			for _, b := range v.branches {
				collectBoundNames(b.body, bound)
			}
			collectBoundNames(v.elseBody, bound)
		case forNode:
			collectBoundNames(v.body, bound)
			collectBoundNames(v.elseBody, bound)
		case caseNode:
			for _, w := range v.whens {
				collectBoundNames(w.body, bound)
			}
			collectBoundNames(v.elseBody, bound)
		case formNode:
			collectBoundNames(v.body, bound)
		}
	}
}

type nameWalker struct {
	src   string
	bound map[string]bool
	loops []string // loop variables in scope, innermost last
	seen  map[string]bool
	out   []NameUse
	// paths reports full paths (ExternalPaths) instead of root names.
	// loopPaths holds, for each loop in loops, the path of the external
	// collection it goes over followed by "[]", or "" when it is not one.
	paths     bool
	loopPaths []string
}

// loopPath returns the collection path a loop variable stands for, and
// whether name is a loop variable at all.
func (w *nameWalker) loopPath(name string) (string, bool) {
	for i := len(w.loops) - 1; i >= 0; i-- {
		if w.loops[i] == name {
			return w.loopPaths[i], true
		}
	}
	return "", false
}

// path is the static dotted path of a variable read, or "" when its root is
// defined by the template.
func (w *nameWalker) path(x varExpr) string {
	root := x.segs[0].name
	base, isLoop := w.loopPath(root)
	if !isLoop {
		if w.defined(root) {
			return ""
		}
		base = root
	} else if base == "" {
		return ""
	}
	for _, seg := range x.segs[1:] {
		if seg.idx != nil {
			break
		}
		base += "." + seg.name
	}
	return base
}

func (w *nameWalker) defined(name string) bool {
	if name == "settings" || w.bound[name] {
		return true
	}
	if name == "forloop" && len(w.loops) > 0 {
		return true
	}
	for _, v := range w.loops {
		if v == name {
			return true
		}
	}
	return false
}

func (w *nameWalker) nodes(nodes []node) {
	for _, n := range nodes {
		switch v := n.(type) {
		case outputNode:
			w.expr(v.expr)
		case assignNode:
			w.expr(v.expr)
		case ifNode:
			for _, b := range v.branches {
				w.expr(b.cond)
				w.nodes(b.body)
			}
			w.nodes(v.elseBody)
		case forNode:
			w.expr(v.coll)
			w.expr(v.limit)
			w.expr(v.offset)
			collection := ""
			if c, ok := v.coll.(varExpr); ok && w.paths {
				if p := w.path(c); p != "" {
					collection = p + "[]"
				}
			}
			w.loops = append(w.loops, v.varName)
			w.loopPaths = append(w.loopPaths, collection)
			w.nodes(v.body)
			w.loops = w.loops[:len(w.loops)-1]
			w.loopPaths = w.loopPaths[:len(w.loopPaths)-1]
			w.nodes(v.elseBody)
		case caseNode:
			w.expr(v.subject)
			for _, when := range v.whens {
				for _, value := range when.values {
					w.expr(value)
				}
				w.nodes(when.body)
			}
			w.nodes(v.elseBody)
		case captureNode:
			w.nodes(v.body)
		case renderNode:
			for _, name := range sortedKeys(v.args) {
				w.expr(v.args[name])
			}
		case formNode:
			for _, name := range sortedKeys(v.attrs) {
				w.expr(v.attrs[name])
			}
			w.nodes(v.body)
		case hostTagNode:
			w.expr(v.arg)
		case cycleNode:
			for _, value := range v.values {
				w.expr(value)
			}
		}
	}
}

func (w *nameWalker) expr(ex expression) {
	switch x := ex.(type) {
	case varExpr:
		if w.paths {
			if p := w.path(x); p != "" && !w.seen[p] {
				w.seen[p] = true
				line, col := lineCol(w.src, x.pos)
				w.out = append(w.out, NameUse{Name: p, Line: line, Col: col})
			}
			for _, seg := range x.segs[1:] {
				if seg.idx != nil {
					w.expr(seg.idx)
				}
			}
			return
		}
		root := x.segs[0].name
		if !w.defined(root) && !w.seen[root] {
			w.seen[root] = true
			line, col := lineCol(w.src, x.pos)
			w.out = append(w.out, NameUse{Name: root, Line: line, Col: col})
		}
		for _, seg := range x.segs[1:] {
			if seg.idx != nil {
				w.expr(seg.idx)
			}
		}
	case binaryExpr:
		w.expr(x.left)
		w.expr(x.right)
	case notExpr:
		w.expr(x.inner)
	case filterExpr:
		w.expr(x.base)
		for _, call := range x.filters {
			for _, arg := range call.args {
				w.expr(arg)
			}
			for _, name := range sortedKeys(call.named) {
				w.expr(call.named[name])
			}
		}
	case rangeExpr:
		w.expr(x.from)
		w.expr(x.to)
	}
}

func sortedKeys(m map[string]expression) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
