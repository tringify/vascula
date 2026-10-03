---
title: Go API
description: Reference for package vascula.dev/vascula — every exported type, function, field and method.
section: Go
order: 30
---

# Go API

```go
import "vascula.dev/vascula"
```

The package has two phases. **Compile** turns source into a `*Template`;
**Render** evaluates a template with `Options`. Everything else configures
one of the two or inspects a compiled template.

## Compiling

### func Compile

```go
func Compile(src string) (*Template, error)
```

Parses `src` with default options: no forms, no host tags, no extra reserved
names, default limits. Equivalent to `CompileWithOptions(src, CompileOptions{})`.

### func CompileWithOptions

```go
func CompileWithOptions(src string, opts CompileOptions) (*Template, error)
```

Parses `src`. Every compilation error carries a line and column. A successful
compile does not guarantee a successful render: data-dependent problems such
as reading a name that is not allowed are found when rendering.

### type CompileOptions

| Field | Meaning |
|---|---|
| `Forms map[string]FormDefinition` | Form actions templates may use with `{% form %}`. See [Extending: forms](/extending#forms). Without it, `{% form %}` fails to compile. |
| `Tags map[string]TagDefinition` | Host tags templates may use. See [Extending: host tags](/extending#host-tags). |
| `Reserved []string` | Names templates cannot bind (assign, capture, loop variable, render argument), in addition to `settings` and `forloop`. Reserve the names of your globals. |
| `MaxSourceBytes int` | Source size limit. Default 1 MiB. |
| `MaxTokens int` | Token limit. Default 100,000. |
| `MaxNesting int` | Nesting limit. Default 128. |

A zero or negative limit means the default. Form definitions are copied when
compiling, so changing the map afterwards does not affect a compiled template.

## Rendering

### func (*Template) Render

```go
func (t *Template) Render(opts Options) (string, error)
```

Renders the template. Safe to call from many goroutines at once on the same
template: a compiled template is never modified.

### type Options

| Field | Meaning |
|---|---|
| `Data map[string]interface{}` | Application data by root name. |
| `Allow []string` | The roots of `Data` the template may read. A dotted entry such as `"cart.items"` allows the whole root `cart`. |
| `Settings map[string]interface{}` | The template's parameters, readable as `settings` without an allowance. |
| `Globals map[string]interface{}` | Names readable everywhere without an allowance, in this template and every child. |
| `Filters map[string]FilterFunc` | Application filters, added to the built-ins. They replace a built-in of the same name, except `escape`, `json` and `default`. |
| `Resolve ResolveChild` | Finds child templates for `{% render %}`. Nil makes `render` fail. |
| `Tags map[string]TagFunc` | Implementations of the host tags declared in `CompileOptions.Tags`. |
| `MaxSteps int` | Work limit. Default 250,000. |
| `MaxBytes int` | Output limit. Default 2 MiB. |
| `MaxValueBytes int` | Size limit for any one value. Default 2 MiB. |
| `MaxValueNodes int` | Item limit for any one value. Default 50,000. |

Limits are shared by the template and all its children. See
[Safety and limits](/safety).

Use JSON-shaped values in `Data`, `Settings` and `Globals`:
`map[string]interface{}`, `[]interface{}`, `string`, `float64` or integers,
`bool`, `nil`, and `SafeHTML` for trusted HTML. `time.Time` works with the
`date` filter. Other types work where they behave like these, but JSON shapes
are what the engine is tuned for.

## Children

### type Child

```go
type Child struct {
	Template *Template
	Allow    []string
	Settings map[string]interface{}
}
```

A child template for `{% render %}`, with its own data allowance and its own
settings. It shares the caller's `Data`, `Globals`, `Filters`, `Tags`,
resolver and limits.

### type ResolveChild

```go
type ResolveChild func(name string) (Child, error)
```

Maps a name used in `{% render "name" %}` to a child. Return an error for an
unknown or forbidden name; it fails the render with your message. Resolve is
called every time a `render` tag runs, so cache compiled children.

```go run
package main

import (
	"fmt"

	"vascula.dev/vascula"
)

func main() {
	children := map[string]*vascula.Template{}
	for name, src := range map[string]string{
		"card":  `<article>{{ product.title }} {% render "price", amount: product.price %}</article>`,
		"price": `<b>{{ amount }}</b>`,
	} {
		t, err := vascula.Compile(src)
		if err != nil {
			panic(err)
		}
		children[name] = t
	}
	page, _ := vascula.Compile(`{% for p in settings.products %}{% render "card", product: p %}{% endfor %}`)
	html, err := page.Render(vascula.Options{
		Settings: map[string]interface{}{"products": []interface{}{
			map[string]interface{}{"title": "Cup", "price": 12},
		}},
		Resolve: func(name string) (vascula.Child, error) {
			t, ok := children[name]
			if !ok {
				return vascula.Child{}, fmt.Errorf("no template named %q", name)
			}
			return vascula.Child{Template: t}, nil
		},
	})
	fmt.Println(html, err)
}
```

```output
<article>Cup <b>12</b></article> <nil>
```

## Filters

### type FilterFunc

```go
type FilterFunc func(input interface{}, args FilterArgs) (interface{}, error)
```

An application filter. Return an error to fail the render; it is reported
with the filter's name and position.

### type FilterArgs

```go
type FilterArgs struct {
	Pos   []interface{}
	Named map[string]interface{}
}

func (a FilterArgs) Arg(i int) interface{}
func (a FilterArgs) NamedArg(name string) interface{}
func (a FilterArgs) HasNamed(name string) bool
```

Positional and named arguments, already evaluated. `Arg` and `NamedArg`
return `nil` for an argument that was not given. Integers that a `float64`
holds exactly arrive as `float64`; larger integers arrive as `int64`.

## Host tags

### type TagDefinition

```go
type TagDefinition struct {
	Argument TagArgument
}
```

Declares a host tag at compilation. `TagArgument` is `TagNoArgument`,
`TagOptionalArgument` or `TagRequiredArgument`. Tag names are lower case
letters, digits and underscores and cannot replace a built-in tag.

### type TagFunc

```go
type TagFunc func(call TagCall) (SafeHTML, error)
```

Renders a host tag. The returned markup is printed as-is.

### type TagCall

```go
type TagCall struct {
	Name   string
	Arg    interface{}
	HasArg bool
}

func (c TagCall) Lookup(name string) (interface{}, bool)
```

`Lookup` reads a name visible at the tag: a variable, `settings`, a global or
an allowed data root. See [Extending: host tags](/extending#host-tags).

## Forms

### type FormDefinition

```go
type FormDefinition struct {
	Path            string
	Multipart       bool
	TokenContextKey string
	TokenField      string
	ActionAttribute string
}
```

A form action for `{% form "kind" %}`. See [Extending: forms](/extending#forms).

## Trusted HTML

### type SafeHTML

```go
type SafeHTML string
```

HTML the application vouches for. A `SafeHTML` value in data or settings, or
returned by a filter or tag, is printed without escaping. Only use it for
markup you produced or sanitized.

## Inspection

| Method | Returns |
|---|---|
| `RenderTargets() []string` | Child names rendered anywhere in the template, in source order, without duplicates. |
| `RenderArguments() map[string][]string` | For each child name, the sorted argument names passed to it. |
| `UsesForm() bool` | Whether the template contains `{% form %}`. |
| `UsesTag(name string) bool` | Whether the template contains the host tag `name`. |
| `ExternalNames(globals ...string) []NameUse` | Every root name the template reads without defining it, excluding the given globals, with the position of its first use. |

All of them visit every branch, including ones that may not run. See
[Inspection](/inspection) for how to use them together.

### type NameUse

```go
type NameUse struct {
	Name      string
	Line, Col int
}
```

## Errors

### type UndeclaredNameError

```go
type UndeclaredNameError struct {
	Name string
}
```

Returned (wrapped) when a template reads a name that is neither a variable nor
an allowed data root. Use `errors.As` to find it, for example to tell the
author which permission to request.

Other errors are plain errors with a readable message ending in
`(line L, col C)` when a position is known. Errors from filters, tags and the
resolver wrap your original error, so `errors.Is` and `errors.As` work on them.
See [Errors](/errors).

### Output observation

`Options.ObserveOutput func(OutputSpan)` optionally receives direct variable outputs
in HTML text. `OutputSpan.Start` and `End` are byte offsets in the returned string;
`Path` identifies the variable and `RootValue` identifies its current root object.
Treat the root as read-only and do not retain it after the render. The observer
does not change rendered bytes. Outputs in attributes, scripts/styles, textarea
and title elements, captures, child renders and filtered expressions are excluded.
This is a host inspection facility, not an authorization decision.
