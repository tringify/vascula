---
title: Errors
description: Every error message, what causes it and how to fix it.
section: Go
order: 35
---

# Errors

Vascula reports mistakes instead of rendering something wrong. Errors come
from two phases:

- **Compile errors** mean the source is malformed. They are found as soon as
  the template is saved, before any data is involved.
- **Render errors** depend on the data: a name that is not allowed, a division
  by zero, a value that is too large.

Messages end with the position, `(line L, col C)`, counting characters from 1,
when one is known. In a child template the message starts with
`{% render %} "name":` and the position is in the child's source.

## Compile errors

### {% x %} is never closed with {% endx %}

A block tag (`if`, `unless`, `for`, `case`, `capture`, `form`, `comment`) has
no matching end tag. The position is where the block opens.

```vascula
<ul>{% for item in settings.items %}<li>{{ item }}</li></ul>
```

```error
{% for %} is never closed with {% endfor %} (line 1, col 5)
```

### unexpected {% endx %} with no opening tag

An end tag, `else`, `elsif` or `when` appears where no block is open, often
because of a typo in the opening tag or an extra end tag.

```vascula
{% if settings.a %}a{% endif %}{% endif %}
```

```error
unexpected {% endif %} with no opening tag
```

### unknown tag "x"

The tag is not part of Vascula and the application did not declare a host tag
with that name.

```vascula
{% includes "footer" %}
```

```error
unknown tag "includes"
```

### expected X, found Y

The tag or expression is malformed at that point. The message names what was
expected and what was found.

```vascula
{% assign total 5 %}
```

```error
expected "=", found number "5" (line 1, col 17)
```

```vascula
{{ product.title | }}
```

```error
expected name, found "}}"
```

### unterminated ...

A string, `{{`, `{%` or `{#` is never closed. Vascula refuses rather than
treating the rest of the file as part of it.

```vascula
<p>{{ "unclosed }}</p>
```

```error
unterminated string
```

### ... is reserved

A name that cannot be bound was assigned, captured, used as a loop variable or
passed to a child: `settings`, `forloop`, or a name the application reserved.
Choose another name.

```vascula
{% for forloop in (1..2) %}{% endfor %}
```

```error
for variable "forloop" is reserved
```

### duplicate render argument

```vascula
{% render "card", title: 1, title: 2 %}
```

```error
duplicate render argument "title"
```

### {% render %} requires a literal template name

The name after `render` must be a quoted string.

```vascula
{% render settings.name %}
```

```error
requires a literal template name
```

### the {% raw %} block is not available

There is no way to print unescaped text from a template. See
[Output](/output).

```vascula
{% raw %}<b>{% endraw %}
```

```error
the {% raw %} block is not available
```

### {% form %} errors

`{% form %}` needs a kind the application registered, and cannot set
attributes the application controls (`action`, `method`, `enctype`). See
[Extending: forms](/extending#forms).

### compile budget exceeded

The source is larger, or nests deeper, than the compilation limits allow. See
[Safety and limits](/safety#compilation-limits).

## Render errors

### undeclared name "x"

The template read a name that is not a variable, not `settings`, not a global
and not an allowed data root. Usually a typo, a missing `assign`, or data the
template must ask the application to allow.

```vascula
{{ customer.name }}
```

```error
undeclared name "customer": it is not a variable or an allowed data root (line 1, col 4)
```

The Go error is a `*vascula.UndeclaredNameError` with the name; see
[Go API](/go-api#type-undeclarednameerror).

### unknown filter "x"

No built-in or application filter has that name.

```vascula
{{ "x" | upcse }}
```

```error
unknown filter "upcse"
```

### filter "x": ...

A filter rejected its input or arguments. The message says why.

```vascula
{{ 10 | divided_by: 0 }}
```

```error
filter "divided_by": divided_by: division by zero
```

```vascula
{{ "tomorrow" | date: "%Y" }}
```

```error
date: cannot parse "tomorrow" as a date
```

### filter "x" does not take a named argument "y"

Built-in filters accept only the named arguments they document (only
`default` has one: `allow_false`). This usually means a filter's arguments
swallowed the next render argument; put the filtered argument last. See
[Composition: arguments](/composition#arguments).

### cannot compare A and B with "op"

`<`, `>`, `<=` and `>=` need two numbers or two strings.

```vascula
{% if settings.count > "3" %}{% endif %}
```

```json data
{"settings": {"count": 5}}
```

```error
cannot compare number and string with ">"
```

### {% break %} outside a for loop

`break` and `continue` only work inside a loop. A child template cannot break
its caller's loop.

```vascula
{% break %}
```

```error
{% break %} outside a for loop
```

### {% render %} ...

A child could not be rendered. The message continues with the reason: the
application's resolver refused the name, the child failed (with its own
message and position), or children nested more than 32 levels deep, which
usually means a template renders itself.

### {% form %}: host token missing

The data root that holds the form token is missing or empty for this render.
The application must supply it.

### render budget exceeded: ...

The render reached one of the [rendering limits](/safety#rendering-limits):

| Message ends with | Meaning |
|---|---|
| `too many steps` | Too much work: usually nested loops over large collections. |
| `output too large` | The page would exceed the output limit. |
| `value too large` | One value has more text than allowed. |
| `too many value nodes` | One value has too many items. |
| `value nesting too deep` | A value is nested more than 64 levels. |
| `range too large` | A range has more items than allowed. |
| `expression too deep` | An expression nests more than 256 levels. |

Reduce the data or the work; raise a limit only if a real template needs it.

### non-finite numbers are not supported

A value was infinite or not a number. Vascula refuses rather than printing
`NaN`.
