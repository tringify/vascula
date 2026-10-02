---
title: Getting started
description: Add Vascula to a Go program, compile a template, render it with data and handle errors.
section: Start
order: 2
---

# Getting started

Vascula is a Go library. This page takes you from nothing to a rendered page.

## Requirements

- Go 1.25 or later.
- Add the module: `go get vascula.dev/vascula@latest`.

The package has no third-party dependencies.

## Render your first template

```go run
package main

import (
	"fmt"

	"vascula.dev/vascula"
)

func main() {
	tmpl, err := vascula.Compile(`<h1>Hello, {{ settings.name }}!</h1>`)
	if err != nil {
		panic(err)
	}
	html, err := tmpl.Render(vascula.Options{
		Settings: map[string]interface{}{"name": "Ada & Grace"},
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(html)
}
```

```output
<h1>Hello, Ada &amp; Grace!</h1>
```

Two steps, always in this order:

1. **Compile** parses the source into a `*Template`. Do it once and keep the
   template: compiling is the expensive part, and a compiled template is safe
   to render from many goroutines at once.
2. **Render** evaluates the template with `Options` and returns the HTML.

`settings` is the template's own parameters. It is always readable, without
any permission.

## Give the template data

Application data goes in `Options.Data`, keyed by root name. A template can
only read the roots listed in `Options.Allow`.

```go run
package main

import (
	"fmt"

	"vascula.dev/vascula"
)

func main() {
	tmpl, err := vascula.Compile(`{% for p in catalog.products %}{{ p.title }} costs {{ p.price }}.
{% endfor %}`)
	if err != nil {
		panic(err)
	}
	html, err := tmpl.Render(vascula.Options{
		Data: map[string]interface{}{
			"catalog": map[string]interface{}{
				"products": []interface{}{
					map[string]interface{}{"title": "Cup", "price": 12},
					map[string]interface{}{"title": "Saucer", "price": 4.5},
				},
			},
			"secrets": "not for templates",
		},
		Allow: []string{"catalog"},
	})
	if err != nil {
		panic(err)
	}
	fmt.Print(html)
}
```

```output
Cup costs 12.
Saucer costs 4.5.
```

`secrets` is in `Data` but not in `Allow`, so the template cannot read it.
If it tried, rendering would fail with an `UndeclaredNameError`.

Use JSON-shaped values: `map[string]interface{}`, `[]interface{}`, strings,
numbers, booleans and `nil`. Data decoded with `encoding/json` already has this
shape.

## Handle errors

Compilation fails on malformed source; rendering fails on things that can only
be known with data, such as reading a name that is not allowed. Both errors say
where the problem is.

```go run
package main

import (
	"errors"
	"fmt"

	"vascula.dev/vascula"
)

func main() {
	_, err := vascula.Compile("<p>\n{% if settings.on %}open")
	fmt.Println(err)

	tmpl, _ := vascula.Compile("<p>{{ user.name }}</p>")
	_, err = tmpl.Render(vascula.Options{})
	fmt.Println(err)

	var undeclared *vascula.UndeclaredNameError
	fmt.Println(errors.As(err, &undeclared), undeclared.Name)
}
```

```output
{% if %} is never closed with {% endif %} (line 2, col 1)
undeclared name "user": it is not a variable or an allowed data root (line 1, col 7)
true user
```

Show these messages to the person who wrote the template. Do not show them, or
the template source, to the people visiting your site. See
[Errors](/errors) for every message and what to do about it.

## Next steps

- Learn the language: [Syntax](/syntax) and [the tutorial](/tutorial).
- Learn the Go API in depth: [Go API](/go-api) and [Embedding](/embedding).
- Add your own filters and tags: [Extending Vascula](/extending).
