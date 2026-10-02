---
title: Safety and limits
description: What Vascula protects against, the resource limits, and what remains the application's responsibility.
section: Go
order: 34
---

# Safety and limits

An application can render templates written by other people with Vascula.
This page says exactly what that covers.

## The model

A **template** is untrusted code. It can be wrong, careless or hostile.

An **application** (the Go program embedding Vascula) is trusted. It chooses
the data, the permissions, the extensions and the limits.

**Data** is untrusted text, whoever wrote it.

Given that, Vascula guarantees:

| Guarantee | How |
|---|---|
| Data cannot inject markup | Every printed value is HTML-escaped. Templates cannot switch it off. See [Output](/output). |
| Data cannot become code | Output is never evaluated again. A value containing `{{ }}` prints those characters. |
| A template reads only what it is given | Only allowed data roots, `settings`, globals and its own variables. Anything else is an error. |
| A template uses only registered extensions | Unknown filters fail the render; unknown tags and form kinds fail compilation. |
| A template cannot exhaust the machine | Compilation and rendering are bounded; see below. |
| A child cannot escape its scope | Children get only their own permissions and the arguments passed to them. |
| Mistakes are reported, not hidden | Undeclared names, bad comparisons, division by zero and broken dates are errors, never silent blanks. |

## Compilation limits

| Limit | Default | Option |
|---|---|---|
| Source size | 1 MiB | `CompileOptions.MaxSourceBytes` |
| Tokens | 100,000 | `CompileOptions.MaxTokens` |
| Nesting of blocks and expressions | 128 | `CompileOptions.MaxNesting` |

## Rendering limits

These cover the whole render: the template and every child it renders.

| Limit | Default | Option |
|---|---|---|
| Work (steps) | 250,000 | `Options.MaxSteps` |
| Output | 2 MiB | `Options.MaxBytes` |
| Size of any one value | 2 MiB of text | `Options.MaxValueBytes` |
| Items in any one value | 50,000 | `Options.MaxValueNodes` |
| Value nesting | 64 levels | fixed |
| Expression depth | 256 | fixed |
| Child render depth | 32 | fixed |

**Work** counts what a render does: each tag, expression, filter call, loop
iteration and every part of a value a filter processes. A render that would
exceed a limit stops with an error that names it:

```vascula
{% for i in (1..40000) %}{% for j in (1..40000) %}{% endfor %}{% endfor %}
```

```error
render budget exceeded: too many steps
```

**Values** are checked against the size limits when they are used: printed,
passed to a filter, compared or serialized. Fields a template never uses are
not checked.

Lower the limits only after testing your real templates; raise them only if
real templates need it.

## What remains your responsibility

- **Permissions.** `Allow` says which roots a template may read; deciding that
  correctly is yours. An allowance covers a whole root.
- **Extensions.** Filters, tags, the resolver and form handling run as your
  code, outside the limits. Keep them fast and bounded, and escape anything
  you put into `SafeHTML`.
- **Trusted HTML.** Only wrap sanitized or self-built markup in `SafeHTML`.
- **URLs, CSS and JavaScript.** Escaping does not make a value a safe URL or
  script; see [Output: what escaping does not cover](/output#what-escaping-does-not-cover).
- **Forms.** The form tag only writes the token; verify it, and validate every
  posted value, on the server.
- **Time and memory.** The limits bound work, not wall-clock time or process
  memory. For templates from untrusted authors, render with a timeout, in a
  process with a memory limit.
- **Error messages.** Show them to authors, not to visitors.

## Reporting a security problem

Send a minimal reproducer, the Vascula version and the options involved to
**developers@tringify.com**. Please do not open a public issue for a
vulnerability.
