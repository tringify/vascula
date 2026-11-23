package vascula

import (
	"fmt"
	"regexp"
)

// TagArgument says whether a host tag takes an expression after its name.
type TagArgument int

const (
	// TagNoArgument: {% name %} only.
	TagNoArgument TagArgument = iota
	// TagOptionalArgument: {% name %} or {% name expression %}.
	TagOptionalArgument
	// TagRequiredArgument: {% name expression %} only.
	TagRequiredArgument
)

// TagDefinition declares a host tag at compilation. A template may use only
// tags its host declared, so an unknown tag fails compilation, not a render.
type TagDefinition struct {
	Argument TagArgument
}

// TagFunc renders a host tag. It returns trusted markup, emitted as-is like
// SafeHTML; it counts toward the output budget. The host enforces its own
// limits for any work it does.
type TagFunc func(call TagCall) (SafeHTML, error)

// TagCall describes one use of a host tag during a render.
type TagCall struct {
	// Name is the tag name.
	Name string
	// Arg is the evaluated argument; HasArg reports whether one was written.
	Arg    interface{}
	HasArg bool
	lookup func(string) (interface{}, bool)
}

// Lookup reads a variable visible at the tag: a local, settings, a global or
// an allowed data root. It reports false for a name the template cannot read.
func (c TagCall) Lookup(name string) (interface{}, bool) {
	if c.lookup == nil {
		return nil, false
	}
	return c.lookup(name)
}

var tagName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// builtinTags cannot be redefined by a host.
var builtinTags = map[string]bool{
	"if": true, "elsif": true, "else": true, "endif": true, "unless": true, "endunless": true,
	"for": true, "endfor": true, "break": true, "continue": true, "case": true, "when": true,
	"endcase": true, "assign": true, "capture": true, "endcapture": true, "cycle": true,
	"render": true, "form": true, "endform": true, "comment": true, "endcomment": true,
	"raw": true, "include": true, "liquid": true, "schema": true, "slot": true,
}

func validateTags(tags map[string]TagDefinition) error {
	for name, def := range tags {
		if !tagName.MatchString(name) {
			return fmt.Errorf("host tag name %q must be lower-case letters, digits and underscores", name)
		}
		if builtinTags[name] {
			return fmt.Errorf("host tag %q would replace a built-in tag", name)
		}
		if def.Argument < TagNoArgument || def.Argument > TagRequiredArgument {
			return fmt.Errorf("host tag %q has an invalid argument rule", name)
		}
	}
	return nil
}

// UndeclaredNameError is a render that read a name which is neither a variable
// nor an allowed data root. Hosts can recognise it with errors.As to explain
// how their template format declares data.
type UndeclaredNameError struct {
	Name string
}

func (e *UndeclaredNameError) Error() string {
	return fmt.Sprintf("undeclared name %q: it is not a variable or an allowed data root", e.Name)
}
