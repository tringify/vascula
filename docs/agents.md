---
title: Vascula for agents
description: A compact, complete brief for AI agents and tools that read or write Vascula templates.
section: Guides
order: 42
---

# Vascula for agents

This page is written for AI agents and code generators, and for people who
want the whole language on one screen. It is complete for writing templates;
the linked pages have the details.

Machine-readable versions: [llms.txt](/llms.txt) (index),
[llms-full.txt](/llms-full.txt) (every page in one file), and any page as
Markdown by adding `.md` to its address, for example [/filters.md](/filters.md).

## Rules that prevent most mistakes

1. **Output is always HTML-escaped.** Never use `raw` or `{% raw %}`; they do
   not exist. Do not call `escape` before printing; it is unnecessary. Only the
   application can print trusted HTML.
2. **Only read names that exist.** A template may read `settings`, its own
   variables, loop variables inside the loop, globals the application defines,
   and the data roots the application allows. Any other name is an error, not
   an empty string. If unsure, ask which roots are available.
3. **`0`, `""` and `[]` are truthy.** Only `false` and `nil` (or missing) are
   false. Test presence with `!= blank` or `.size > 0`.
4. **There is no `not` and no grouping parentheses.** Use `unless` or `!=`.
   `a or b and c` means `a or (b and c)`.
5. **There are no arithmetic operators or array literals.** Use filters
   (`plus`, `minus`, `times`, `divided_by`, `modulo`) and ranges `(1..n)`.
6. **A filter argument is one value** (literal, variable or range). Assign
   intermediate results to variables.
7. **A loop collection cannot take filters.** `{% assign s = list | sort %}`
   then `{% for x in s %}`.
8. **`render` names are quoted strings**, and the child sees only its
   arguments. Put a filtered argument last: `{% render "c", a: 1, b: x | plus: 1 %}`.
9. **Ordering comparisons need matching types.** `count > "3"` is an error;
   `"5" == 5` is true.
10. **`divided_by` keeps fractions:** `7 | divided_by: 2` is `3.5`. Add
    `| floor` for integer division.
11. **Use `json` to put data in `<script>` or in an attribute.** It is safe in
    both. Never print values into `onclick`, `style` or a URL without the
    application validating them.
12. **Variables are template-wide.** A value assigned inside `if` or `for` is
    available after it. Loop variables are not.

## Syntax at a glance

```vascula fragment
{{ expression }}                 output (escaped)
{{ value | filter: arg, name: arg | filter2 }}
{%- tag -%}                       - trims whitespace on that side
{# comment #}  {% comment %}...{% endcomment %}

{% if cond %}...{% elsif cond %}...{% else %}...{% endif %}
{% unless cond %}...{% else %}...{% endunless %}
{% case value %}{% when a, b %}...{% when c or d %}...{% else %}...{% endcase %}
{% for item in collection limit: n offset: n reversed %}...{% else %}...{% endfor %}
{% break %}  {% continue %}  {% cycle "a", "b" %}
{% assign name = expression %}
{% capture name %}...{% endcapture %}
{% render "child", arg: value %}
{% form "kind", class: "x" %}...{% endform %}      (application-defined kinds)
```

Values: `"string"`, `'string'`, `42`, `-1.5`, `true`, `false`, `nil`,
`empty`/`blank`, `(1..5)`. Paths: `a.b`, `a["b"]`, `a[0]`, `a[-1]`,
`a.size`, `a.first`, `a.last`. Operators: `==`, `!=`, `<`, `>`, `<=`, `>=`,
`contains`, `and`, `or`.

`forloop`: `index`, `index0`, `rindex`, `rindex0`, `first`, `last`,
`length`, `parentloop`.

## Built-in filters

| Group | Filters |
|---|---|
| Strings | `upcase`, `downcase`, `capitalize`, `strip`, `lstrip`, `rstrip`, `strip_newlines`, `append`, `prepend`, `replace`, `replace_first`, `remove`, `remove_first`, `truncate`, `truncatewords`, `split`, `slice`, `size` |
| HTML | `escape`, `escape_once`, `strip_html`, `newline_to_br` |
| Numbers | `plus`, `minus`, `times`, `divided_by`, `modulo`, `round`, `ceil`, `floor`, `abs`, `at_least`, `at_most` |
| Arrays | `join`, `first`, `last`, `map`, `where`, `sort`, `sort_natural`, `reverse`, `uniq`, `compact`, `concat`, `sum`, `slice`, `size` |
| Other | `date`, `url_encode`, `url_decode`, `default` (named: `allow_false`), `json` |

Applications add their own filters (for example `money`, `image_url`, `t`).
Only use filters you know the application provides.

## Checking your work

A template that compiles has valid syntax; compile errors give a line and
column (see [Errors](/errors)). If you have access to the application's
tooling, compile the template and list its external names to confirm it only
reads available roots (see [Inspection](/inspection)).

## A complete, idiomatic example

```vascula
{%- assign visible = settings.products | where: "available" -%}
<section class="products">
  <h2>{{ settings.heading | default: "Products" }}</h2>
  {%- if visible.size > 0 %}
  <ul>
    {%- for product in visible limit: settings.limit %}
    <li{% if forloop.first %} class="first"{% endif %}>{{ product.title }}
      {%- if product.tags.size > 0 %} ({{ product.tags | join: ", " }}){% endif -%}
    </li>
    {%- endfor %}
  </ul>
  {%- else %}
  <p>Nothing available right now.</p>
  {%- endif %}
</section>
```

```json data
{"settings": {"heading": "", "limit": 2, "products": [
  {"title": "Sencha", "available": true, "tags": ["green"]},
  {"title": "Assam", "available": false, "tags": []},
  {"title": "Oolong", "available": true, "tags": []},
  {"title": "Pu-erh", "available": true, "tags": ["aged"]}
]}}
```

```output
<section class="products">
  <h2>Products</h2>
  <ul>
    <li class="first">Sencha (green)</li>
    <li>Oolong</li>
  </ul>
</section>
```
