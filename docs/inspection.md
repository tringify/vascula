---
title: Inspection
description: Check a template before it renders — the names it reads, the children it renders and the features it uses.
section: Go
order: 33
---

# Inspection

A compiled template can tell you, without rendering it, what it will need.
Use this when a template is saved or published, so mistakes are reported to
its author instead of failing a live page.

All inspection methods look at every branch, including ones that may never
run, and none of them needs data.

## The names a template reads

`ExternalNames` lists every root name the template reads without defining it
itself, with where it is first used. Variables it assigns or captures, loop
variables inside their loops, `forloop` inside loops and `settings` are not
external. Pass the names of your globals to exclude them too.

```go run
package main

import (
	"fmt"

	"vascula.dev/vascula"
)

func main() {
	tmpl, err := vascula.Compile(`{% assign n = cart.items.size %}
{{ site.name }}: {{ n }} items{% if customer %} for {{ customer.name }}{% endif %}
{% for item in cart.items %}{{ item.title }}{{ settings.separator }}{% endfor %}`)
	if err != nil {
		panic(err)
	}
	for _, use := range tmpl.ExternalNames("site") {
		fmt.Printf("%s (line %d, col %d)\n", use.Name, use.Line, use.Col)
	}
}
```

```output
cart (line 1, col 15)
customer (line 2, col 37)
```

Compare the result with what the template will be given. Anything left over
will fail the first render that reaches it:

```go
allowed := map[string]bool{"cart": true} // the roots this template declared
for _, use := range tmpl.ExternalNames("site") {
	if !allowed[use.Name] {
		return fmt.Errorf("line %d: %q is not available here", use.Line, use.Name)
	}
}
```

One case it cannot catch: reading a variable before the `assign` that creates
it has run. That is still found when rendering.

## The composition tree

`RenderTargets` lists every child name used with `{% render %}`, in order,
without duplicates. Because names must be literal strings, this is the
complete list.

`RenderArguments` lists, for each child, the argument names passed to it. A
child's `ExternalNames` must be covered by its own allowance plus the
arguments its callers pass.

```go run
package main

import (
	"fmt"

	"vascula.dev/vascula"
)

func main() {
	page, _ := vascula.Compile(`{% render "header" %}{% for p in catalog.products %}{% render "card", product: p, compact: true %}{% endfor %}`)
	card, _ := vascula.Compile(`<h2>{{ product.title }}</h2>{% unless compact %}{{ product.text }}{% endunless %}{{ shop.currency }}`)

	fmt.Println(page.RenderTargets())
	args := page.RenderArguments()
	fmt.Println(args["card"])

	passed := map[string]bool{}
	for _, a := range args["card"] {
		passed[a] = true
	}
	for _, use := range card.ExternalNames() {
		if !passed[use.Name] {
			fmt.Println("card needs an allowance for", use.Name)
		}
	}
}
```

```output
[header card]
[compact product]
card needs an allowance for shop
```

Walk the tree from a page through `RenderTargets` of each child to find
missing children and cycles before anything renders.

## Features used

`UsesForm` reports whether a template contains `{% form %}`, and
`UsesTag(name)` whether it uses a host tag. Use them to check a template only
uses what its context supports, or to prepare work only when needed (for
example, issue a form token only for pages with forms).
