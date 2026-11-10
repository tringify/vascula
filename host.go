package vascula

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// FormDefinition is a trusted host action bound into a compiled template.
// Templates cannot choose arbitrary action URLs. Path must be a local absolute
// POST path, and TokenContextKey and TokenField must be set. Rendering the token
// field does not authenticate a request; the host issues and verifies tokens.
// Hosts must version their form profile with their template compilation cache.
type FormDefinition struct {
	Path            string
	Multipart       bool
	TokenContextKey string
	TokenField      string
	ActionAttribute string
}

// CompileOptions supplies the host capabilities allowed during compilation.
// With no Forms, form tags fail compilation. Definitions are snapshotted into
// the compiled template, so later map changes cannot change an already compiled
// action.
type CompileOptions struct {
	Forms map[string]FormDefinition
	// Tags declares host tags templates may use; Options.Tags implements them.
	Tags map[string]TagDefinition
	// Reserved names cannot be assigned, captured, used as a loop variable or
	// passed as a render argument, in addition to settings and forloop. Reserve
	// the names of your Globals so a template cannot shadow them.
	Reserved []string
	// Nonpositive limits use defaults: 1 MiB source, 100,000 tokens and 128
	// nested constructs. A host must include its compile policy in cache identity.
	MaxSourceBytes int
	MaxTokens      int
	MaxNesting     int
}

var attributeName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

func (d FormDefinition) validate() error {
	u, err := url.Parse(d.Path)
	if err != nil || !strings.HasPrefix(d.Path, "/") || strings.HasPrefix(d.Path, "//") || strings.ContainsAny(d.Path, "\\\r\n") || u.Host != "" || u.Fragment != "" {
		return fmt.Errorf("form path must be a local absolute path")
	}
	if d.TokenContextKey == "" || d.TokenField == "" {
		return fmt.Errorf("form requires token context key and token field")
	}
	if d.ActionAttribute != "" && (!attributeName.MatchString(d.ActionAttribute) || !strings.HasPrefix(d.ActionAttribute, "data-")) {
		return fmt.Errorf("form action attribute must be a data attribute")
	}
	return nil
}
