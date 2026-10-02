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
			w.loops = append(w.loops, v.varName)
			w.nodes(v.body)
			w.loops = w.loops[:len(w.loops)-1]
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
