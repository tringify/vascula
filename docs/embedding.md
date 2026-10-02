---
title: Embedding
description: How to design the data a template sees, cache compiled templates, compose children and report errors in a Go application.
section: Go
order: 31
---

# Embedding Vascula

This page is about the decisions you make when you build Vascula into an
application. The exact types are in the [Go API](/go-api) reference.

## Data and Allow

Templates get data in three ways, and each has a purpose:

| Option | Readable as | Use it for |
|---|---|---|
| `Settings` | `settings` | A template's own parameters: a heading, a number of columns, a chosen colour. |
| `Globals` | the key's name | Things every template may read: site-wide configuration, the current locale. |
| `Data` + `Allow` | the key's name, **if allowed** | Application data with access control: the signed-in user, the cart, an order. |

`Allow` is the template's permission list. Build `Data` with everything the
page might need, and give each template only the roots it should read. A
template that is not allowed `customer` cannot read the customer, even though
it is in `Data`.

```go
html, err := tmpl.Render(vascula.Options{
	Data: map[string]interface{}{
		"product":  product,  // public
		"customer": customer, // personal
	},
	Allow:    []string{"product"},             // this template may not read customer
	Settings: map[string]interface{}{"columns": 3},
	Globals:  map[string]interface{}{"site": siteConfig},
})
```

An allowance is for a whole root: `"cart.items"` allows all of `cart`. Do not
put anything in a root that some template allowed to read it must not see.

**Let templates declare what they need.** A good pattern is to store, with each
template, the list of roots it reads, and to pass that list as `Allow`. Check
the list when the template is saved with [`ExternalNames`](/inspection), so a
template cannot be saved if it reads something it did not declare.

**Build only what was asked for.** Since you know each template's roots before
rendering, you can skip loading data no template on the page reads.

## Reserve your globals

A template could `assign` a name that hides a global. Reserve each global's
name at compilation so that is a compilation error:

```go
tmpl, err := vascula.CompileWithOptions(src, vascula.CompileOptions{
	Reserved: []string{"site"},
})
```

## Cache compiled templates

Compile when a template is saved or first used, and keep the `*Template`.
Rendering a compiled template is cheap; compiling it on every request is not.

A compiled template depends on its source **and** on the `CompileOptions` you
compiled it with (forms, tags, reserved names, limits). Key your cache on all
of them, or on a version you change when you change the options. Never key it
on `Settings` or `Data`: those are render inputs and change per request.

A compiled template is immutable and safe to share between goroutines.

## Children

`Resolve` turns a name in `{% render "name" %}` into a `Child`: a compiled
template with its own `Allow` and `Settings`. Typical choices:

- Look the name up in a registry of compiled templates.
- Give each child the allowance stored with it, not the caller's. A child is
  a peer with its own permissions.
- Return an error for names a template may not use. The render fails with your
  message.

Before rendering, you can walk the whole tree with
[`RenderTargets`](/inspection#the-composition-tree): every name is a literal,
so the tree is known in advance, and you can reject cycles or missing children
when a template is saved.

## Limits and isolation

Every render is bounded (see [Safety and limits](/safety)). The limits stop a
template from looping forever or producing huge output, and they apply to the
page as a whole, children included. They do not limit time spent inside your
own filters, tags and resolver: bound those yourself.

If you render templates written by people you do not trust, run rendering with
a timeout and, ideally, in a process with its own memory limit.

## Showing errors

Errors are written for the template's author: they name the problem and the
line and column. Show them in your editor or preview. Do not show them, or the
template source, to the visitors of a live page; render a generic error page
instead and log the message.

To give authors better guidance, check for
[`UndeclaredNameError`](/go-api#type-undeclarednameerror) and explain how
they request access to a root in your system.

## Rendering into a page

Vascula renders strings. Set the response's content type yourself
(`text/html; charset=utf-8` for HTML), and write the result as-is: it is
already escaped.

Never feed rendered output back into `Compile`. Output is data, not template
source.
