package vascula

import "sort"

// RenderTargets returns literal child names in first source order, without
// duplicates. It visits every control-flow body, including branches that will
// not execute. Dynamic targets fail compilation. The result contains direct
// dependencies; it does not resolve or inspect the children themselves.
func (t *Template) RenderTargets() []string {
	seen := map[string]bool{}
	var out []string
	collectRenderTargets(t.nodes, seen, &out)
	return out
}

func collectRenderTargets(nodes []node, seen map[string]bool, out *[]string) {
	for _, n := range nodes {
		switch v := n.(type) {
		case renderNode:
			if name, ok := literalString(v.target); ok && !seen[name] {
				seen[name] = true
				*out = append(*out, name)
			}
		case ifNode:
			for _, b := range v.branches {
				collectRenderTargets(b.body, seen, out)
			}
			collectRenderTargets(v.elseBody, seen, out)
		case forNode:
			collectRenderTargets(v.body, seen, out)
			collectRenderTargets(v.elseBody, seen, out)
		case formNode:
			collectRenderTargets(v.body, seen, out)
		case captureNode:
			collectRenderTargets(v.body, seen, out)
		case caseNode:
			for _, branch := range v.whens {
				collectRenderTargets(branch.body, seen, out)
			}
			collectRenderTargets(v.elseBody, seen, out)
		}
	}
}

// UsesForm reports whether the template statically contains a {% form %} tag (recursing into
// all control-flow bodies). Hosts can use this to prepare the configured form token.
func (t *Template) UsesForm() bool {
	return nodesUseForm(t.nodes)
}

func nodesUseForm(nodes []node) bool {
	for _, n := range nodes {
		switch v := n.(type) {
		case formNode:
			return true
		case ifNode:
			for _, b := range v.branches {
				if nodesUseForm(b.body) {
					return true
				}
			}
			if nodesUseForm(v.elseBody) {
				return true
			}
		case forNode:
			if nodesUseForm(v.body) || nodesUseForm(v.elseBody) {
				return true
			}
		case captureNode:
			if nodesUseForm(v.body) {
				return true
			}
		case caseNode:
			for _, branch := range v.whens {
				if nodesUseForm(branch.body) {
					return true
				}
			}
			if nodesUseForm(v.elseBody) {
				return true
			}
		}
	}
	return false
}

// UsesTag reports whether the template statically contains the host tag
// name, visiting every control-flow body. Hosts may use it to validate
// capabilities before rendering.
func (t *Template) UsesTag(name string) bool {
	return nodesUseTag(t.nodes, name)
}

func nodesUseTag(nodes []node, name string) bool {
	for _, n := range nodes {
		switch v := n.(type) {
		case hostTagNode:
			if v.name == name {
				return true
			}
		case ifNode:
			for _, b := range v.branches {
				if nodesUseTag(b.body, name) {
					return true
				}
			}
			if nodesUseTag(v.elseBody, name) {
				return true
			}
		case forNode:
			if nodesUseTag(v.body, name) || nodesUseTag(v.elseBody, name) {
				return true
			}
		case formNode:
			if nodesUseTag(v.body, name) {
				return true
			}
		case captureNode:
			if nodesUseTag(v.body, name) {
				return true
			}
		case caseNode:
			for _, branch := range v.whens {
				if nodesUseTag(branch.body, name) {
					return true
				}
			}
			if nodesUseTag(v.elseBody, name) {
				return true
			}
		}
	}
	return false
}

// RenderArguments returns, for each literal render target, the sorted names
// of every argument passed to it anywhere in the template. A host combines it
// with a child's ExternalNames: what a child reads without defining it must
// be an allowed root or an argument every caller can pass.
func (t *Template) RenderArguments() map[string][]string {
	sets := map[string]map[string]bool{}
	collectRenderArguments(t.nodes, sets)
	out := make(map[string][]string, len(sets))
	for target, names := range sets {
		list := make([]string, 0, len(names))
		for name := range names {
			list = append(list, name)
		}
		sort.Strings(list)
		out[target] = list
	}
	return out
}

func collectRenderArguments(nodes []node, sets map[string]map[string]bool) {
	for _, n := range nodes {
		switch v := n.(type) {
		case renderNode:
			if name, ok := literalString(v.target); ok {
				if sets[name] == nil {
					sets[name] = map[string]bool{}
				}
				for arg := range v.args {
					sets[name][arg] = true
				}
			}
		case ifNode:
			for _, b := range v.branches {
				collectRenderArguments(b.body, sets)
			}
			collectRenderArguments(v.elseBody, sets)
		case forNode:
			collectRenderArguments(v.body, sets)
			collectRenderArguments(v.elseBody, sets)
		case formNode:
			collectRenderArguments(v.body, sets)
		case captureNode:
			collectRenderArguments(v.body, sets)
		case caseNode:
			for _, branch := range v.whens {
				collectRenderArguments(branch.body, sets)
			}
			collectRenderArguments(v.elseBody, sets)
		}
	}
}
