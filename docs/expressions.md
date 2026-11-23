---
title: Expressions and operators
description: Filters, comparisons, and/or, ranges, and the order in which they apply.
section: Language
order: 13
---

# Expressions and operators

An expression is what goes inside `{{ }}`, after `if` or `elsif`, after
`assign name =`, and in a few other tag positions.

## Filters

A filter transforms the value on its left. Chain filters with `|`; they apply
left to right. Arguments follow a colon, separated by commas.

```vascula
{{ "  hello world  " | strip | capitalize | append: "!" }}
{{ "a,b,c" | split: "," | join: " + " }}
{{ settings.text | truncate: 8, "…" }}
```

```json data
{"settings": {"text": "Vascula templates"}}
```

```output
Hello world!
a + b + c
Vascula…
```

Some filters also take **named** arguments, written `name: value` after any
positional ones:

```vascula
{{ settings.on | default: true, allow_false: true }}
```

```json data
{"settings": {"on": false}}
```

```output
false
```

A filter argument is a single value: a literal, a variable path or a range.
To pass the result of another filter, assign it first.

```vascula
{% assign suffix = settings.unit | upcase %}{{ settings.amount | append: suffix }}
```

```json data
{"settings": {"amount": 3, "unit": "kg"}}
```

```output
3KG
```

Every built-in filter is described in [Filters](/filters). Applications add
their own, such as formatting money or building image URLs.

## Comparisons

`==`, `!=`, `<`, `>`, `<=`, `>=` and `contains` compare two values. The rules
for each are in [Values: comparing values](/values#comparing-values). Filters
bind tighter than comparisons, so each side is filtered first:

```vascula
{% if settings.name | downcase == "ada" %}hello Ada{% endif %}
```

```json data
{"settings": {"name": "ADA"}}
```

```output
hello Ada
```

A comparison takes exactly two sides: `a == b == c` is a compilation error.

## and, or

`and` and `or` combine conditions. They **return one of their operands**,
not a new `true` or `false`:

- `a or b` is `a` when `a` is truthy, otherwise `b`.
- `a and b` is `b` when `a` is truthy, otherwise `a`.

That makes `or` a convenient fallback:

```vascula
{{ settings.nickname or settings.name }} / {{ settings.missing or "Guest" }} / {{ settings.name and "has a name" }}
```

```json data
{"settings": {"name": "Ada"}}
```

```output
Ada / Guest / has a name
```

The right side is only evaluated when needed, so `customer and customer.name`
never reads a field of nothing.

## Precedence

From tightest to loosest:

1. A value with its filters: `x | upcase`
2. A comparison: `a == b`
3. `and`
4. `or`

There are no parentheses for grouping, so `a or b and c` means
`a or (b and c)`. For anything more involved, assign intermediate results to
variables. Parentheses are only used for [ranges](/loops#ranges).

```vascula
{% if false or true and false %}yes{% else %}no{% endif %}
```

```output
no
```

## There is no `not`

Use `unless`, `!=`, or `== false`:

```vascula
{% unless settings.hidden %}shown{% endunless %} {% if settings.count != 0 %}nonzero{% endif %}
```

```json data
{"settings": {"hidden": false, "count": 2}}
```

```output
shown nonzero
```

## Ranges

`(from..to)` is the list of integers from `from` to `to`, inclusive. Each end
is a number or a variable. A range whose end is below its start is empty.

```vascula
{{ (1..4) | join: "," }} | {% for i in (1..settings.n) %}{{ i }}{% endfor %} | [{{ (3..1) | join: "," }}]
```

```json data
{"settings": {"n": 3}}
```

```output
1,2,3,4 | 123 | []
```

Next: [Conditionals](/conditionals).
