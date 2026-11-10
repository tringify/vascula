---
title: Values and truthiness
description: Strings, numbers, booleans, nil, collections, what counts as true, and how values compare.
section: Language
order: 11
---

# Values and truthiness

## Kinds of values

| Kind | Written as | Notes |
|---|---|---|
| String | `"text"` or `'text'` | A backslash takes the next character literally: `"say \"hi\""`. There are no escape sequences such as `\n`. |
| Integer | `42`, `-7` | Exact 64-bit integers. |
| Decimal | `3.14`, `-0.5` | 64-bit floating point. Exponent notation is not accepted. |
| Boolean | `true`, `false` | |
| Nothing | `nil` or `null` | Prints nothing. |
| Empty | `empty` or `blank` | Only for comparisons; see [Empty](#empty-and-blank). |
| Range | `(1..5)` | A list of integers; see [Loops](/loops#ranges). |
| Array, map | from data only | There are no array or map literals. |

```vascula
{{ "double" }} {{ 'single' }} {{ "say \"hi\"" }} {{ 42 }} {{ -0.5 }} {{ true }} [{{ nil }}]
```

```output
double single say &#34;hi&#34; 42 -0.5 true []
```

### Integers stay exact

A number without a decimal point is an exact integer, and arithmetic between
two integers stays exact. This matters for large identifiers.

```vascula
{{ 9007199254740993 | plus: 1 }} {{ 10 | divided_by: 4 }} {{ 10 | divided_by: 5 }}
```

```output
9007199254740994 2.5 2
```

Division keeps a fractional result. A calculation that overflows 64 bits
falls back to floating point. Data from the application keeps its own type:
numbers decoded from JSON are floating point.

## Truthiness

Conditions such as `{% if %}` test whether a value is *truthy*.

| Value | In a condition |
|---|---|
| `false` | false |
| `nil`, a missing field | false |
| `true` | true |
| `0`, `0.0` | **true** |
| `""` (empty string) | **true** |
| `[]`, `{}` (empty array or map) | **true** |
| any other value | true |

Only `false` and nothing are false. To ask "is there anything here?", compare
with `blank`, or check `.size`.

```vascula
{% if settings.zero %}0 is true{% endif %}, {% if settings.text %}"" is true{% endif %}, {% if settings.missing %}{% else %}missing is false{% endif %}
```

```json data
{"settings": {"zero": 0, "text": ""}}
```

```output
0 is true, "" is true, missing is false
```

## Empty and blank

`empty` and `blank` are the same marker. A value equals it when the value is
nothing, an empty string, or a collection with no entries. A string of spaces
is **not** blank.

```vascula
{% if settings.title == blank %}No title{% endif %} / {% if settings.tags == empty %}No tags{% endif %} / {% if settings.space == blank %}blank{% else %}not blank{% endif %}
```

```json data
{"settings": {"title": "", "tags": [], "space": " "}}
```

```output
No title / No tags / not blank
```

## Comparing values

| Operator | Meaning |
|---|---|
| `==`, `!=` | Equal, not equal. |
| `<`, `>`, `<=`, `>=` | Order. Both sides must be numbers, or both strings. |
| `contains` | Substring of a string, or member of an array. |

Equality is forgiving: numbers compare as numbers, and a string that is a
number equals that number. Anything else compares by its printed text.

```vascula
{{ 1 == 1.0 }} {{ "5" == 5 }} {{ "a" == "a" }} {{ nil == nil }} {{ "1" != 2 }}
```

```output
true true true true true
```

Ordering is strict. Comparing a number with a string, or anything that is not
a number or string, is an error, never a silent `false`.

```vascula
{{ 2 > 1 }} {{ "b" > "a" }} {{ 10 >= 10 }}
```

```output
true true true
```

```vascula
{% if settings.count > "3" %}more{% endif %}
```

```json data
{"settings": {"count": 5}}
```

```error
cannot compare number and string with ">"
```

`contains` looks for a substring in a string, or an equal member in an array.
It is always false for maps and other values.

```vascula
{{ "teapot" contains "tea" }} {{ settings.tags contains "new" }} {{ settings.tags contains "old" }}
```

```json data
{"settings": {"tags": ["new", "sale"]}}
```

```output
true true false
```

## How values print

| Value | Prints as |
|---|---|
| String | itself, HTML-escaped |
| Integer | digits: `42` |
| Decimal | shortest exact form: `2.5`, `3` for `3.0` |
| Boolean | `true` or `false` |
| `nil` | nothing |
| Array | its items printed one after another, with no separator; use `join` |
| Map | a debugging form; print its fields instead |

```vascula
{{ settings.list }} | {{ settings.list | join: ", " }} | {{ 3.0 }}
```

```json data
{"settings": {"list": ["a", "b", "c"]}}
```

```output
abc | a, b, c | 3
```

Next: [Variables, data and scope](/variables).
