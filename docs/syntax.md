---
title: Syntax
description: The three kinds of markup in a template, whitespace control and comments.
section: Language
order: 10
---

# Syntax

A template is text with three kinds of markup in it:

| Markup | Meaning |
|---|---|
| `{{ expression }}` | **Output.** Evaluate the expression and print it, HTML-escaped. |
| `{% tag ... %}` | **Tag.** Logic: conditions, loops, variables, composition. Prints nothing itself. |
| `{# comment #}` | **Comment.** Removed from the output. |

Everything else is literal text and is printed exactly as written.

```vascula
<p>Hello, {{ settings.name }}.</p>{% if settings.vip %} <strong>Welcome back!</strong>{% endif %}{# greeting #}
```

```json data
{"settings": {"name": "Ada", "vip": true}}
```

```output
<p>Hello, Ada.</p> <strong>Welcome back!</strong>
```

## Output

`{{ }}` takes one expression: a value, optionally passed through filters with
`|`.

```vascula
{{ "vascula" }} {{ 42 }} {{ settings.name }} {{ settings.name | upcase }}
```

```json data
{"settings": {"name": "Ada"}}
```

```output
vascula 42 Ada ADA
```

Output is always escaped for HTML. To learn what that covers and what it does
not, see [Output and escaping](/output).

## Tags

Tags start with a keyword. Some stand alone (`assign`, `render`, `cycle`,
`break`), and some wrap a body that ends with a matching `end` tag (`if` …
`endif`, `for` … `endfor`, `case` … `endcase`, `capture` … `endcapture`).

```vascula
{% assign greeting = "Hi" %}{% for n in (1..3) %}{{ greeting }} {{ n }}. {% endfor %}
```

```output
Hi 1. Hi 2. Hi 3. 
```

The full list is on the pages for [variables](/variables),
[conditionals](/conditionals), [loops](/loops) and
[composition](/composition). An unknown tag is a compilation error.

## Whitespace control

Tags leave the line breaks around them in the output. Put `-` just inside a
delimiter to remove all whitespace, including newlines, on that side.

```vascula
<ul>
  {%- for x in (1..2) %}
  <li>{{ x }}</li>
  {%- endfor %}
</ul>
```

```output
<ul>
  <li>1</li>
  <li>2</li>
</ul>
```

`{%-` and `{{-` trim before the tag; `-%}` and `-}}` trim after it. Comments
accept `{#-` and `-#}` the same way.

## Comments

`{# ... #}` is removed from the output and can span lines. For a longer
comment, or to switch off a piece of template while you work, use the
`comment` block: everything up to `{% endcomment %}` is ignored, including
tags.

```vascula
Visible{# not visible #}{% comment %}{% if broken %}ignored{% endcomment %}.
```

```output
Visible.
```

An unterminated comment is a compilation error rather than a comment that
silently swallows the rest of the template.

## Names

Names are made of letters, digits and underscores and start with a letter or
underscore: `product`, `item_count`, `_draft`. Names are case-sensitive.

## Text is never evaluated

Text that comes from data is printed, never run. If a value contains
`{{ something }}`, the page shows those characters.

```vascula
{{ settings.note }}
```

```json data
{"settings": {"note": "{{ settings.secret }}"}}
```

```output
{{ settings.secret }}
```

## What is not supported

These are compilation errors, so a mistake never reaches a live page:

| You may know | In Vascula |
|---|---|
| `{% raw %}` and the `raw` filter | Not available. Output is always escaped. |
| `{% include %}` | Use `{% render %}`; see [Composition](/composition). |
| `{% liquid %}`, `{% schema %}`, `{% javascript %}`, `{% stylesheet %}`, `{% section %}` | Not part of the language. An application can add its own tags; see [Extending Vascula](/extending). |
| Arithmetic operators (`a + b`) | Use filters: `{{ a \| plus: b }}`. |
| Array literals (`[1, 2]`) | Use a range `(1..3)` or `split`. |

The next page covers the values you can write and how they behave:
[Values and truthiness](/values).
