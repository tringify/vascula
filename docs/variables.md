---
title: Variables, data and scope
description: Where a template's data comes from, reading fields, assign and capture, and the scope rules.
section: Language
order: 12
---

# Variables, data and scope

## Where names come from

When a template reads a name such as `product` in `{{ product.title }}`,
Vascula looks it up in this order:

1. **Variables** the template created with `assign` or `capture`, and loop
   variables inside their loop.
2. **`settings`**: the template's own parameters. Always readable.
3. **Globals**: names the application makes readable everywhere, such as
   `theme` in a theme system.
4. **Data roots** the application allowed for this template.

Anything else is an error, so a typo cannot quietly print nothing:

```vascula
{{ produt.title }}
```

```json data
{"data": {"product": {"title": "Cup"}}, "allow": ["product"]}
```

```error
undeclared name "produt"
```

The application decides which data roots a template may read (see
[Embedding](/embedding#data-and-allow)). A root in the data but not allowed is
just as unreadable as a name that does not exist.

## Reading fields

Use a dot for a field and brackets for a key or position.

```vascula
{{ product.title }} / {{ product["title"] }} / {{ product.tags[0] }} / {{ product.tags[-1] }} / {{ product.tags[settings.i] }}
```

```json data
{"data": {"product": {"title": "Cup", "tags": ["new", "sale", "blue"]}}, "allow": ["product"], "settings": {"i": 1}}
```

```output
Cup / Cup / new / blue / sale
```

A negative position counts from the end. A missing field or an out-of-range
position is `nil`, which prints nothing; only the first name of a path must
exist.

```vascula
[{{ product.missing }}] [{{ product.missing.deeper }}] [{{ product.tags[9] }}]
```

```json data
{"data": {"product": {"tags": ["new"]}}, "allow": ["product"]}
```

```output
[] [] []
```

### size, first and last

Every array, map and string has three built-in properties. A field of the same
name in a map wins.

| Property | Array | Map | String |
|---|---|---|---|
| `.size` | number of items | number of keys | number of characters |
| `.first` | first item | `nil` | `nil` |
| `.last` | last item | `nil` | `nil` |

```vascula
{{ product.tags.size }} {{ product.tags.first }} {{ product.tags.last }} {{ product.title.size }}
```

```json data
{"data": {"product": {"title": "Café", "tags": ["new", "sale"]}}, "allow": ["product"]}
```

```output
2 new sale 4
```

`.size` of something missing is `0`, so `{% if product.images.size > 0 %}` is
safe even when `images` is absent.

## assign

`assign` stores the value of an expression, filters included.

```vascula
{% assign title = product.title | upcase %}{% assign count = product.tags.size %}{{ title }} has {{ count }} tags.
```

```json data
{"data": {"product": {"title": "Cup", "tags": ["new", "sale"]}}, "allow": ["product"]}
```

```output
CUP has 2 tags.
```

## capture

`capture` renders its body and stores the result as a string. The stored
string is ordinary text: printing it escapes it again.

```vascula
{% capture label %}{{ product.title }} ({{ product.tags.size }}){% endcapture %}<span title="{{ label }}">{{ label }}</span>
```

```json data
{"data": {"product": {"title": "Cup", "tags": ["new"]}}, "allow": ["product"]}
```

```output
<span title="Cup (1)">Cup (1)</span>
```

For reusable markup, prefer [`render`](/composition) to `capture`.

## Scope

A template has one set of variables. A value assigned or captured anywhere,
including inside `if`, `case`, `for` or `capture`, is available after that
block ends.

```vascula
{% for n in (1..5) %}{% if n > 2 %}{% assign first_big = first_big | default: n %}{% endif %}{% endfor %}{{ first_big }}
```

```error
undeclared name "first_big"
```

Reading a variable before anything has assigned it is still an error, as the
example shows: the first `default` reads `first_big` before it exists. Assign
a starting value first:

```vascula
{% assign first_big = nil %}{% for n in (1..5) %}{% if n > 2 %}{% assign first_big = first_big | default: n %}{% endif %}{% endfor %}{{ first_big }}
```

```output
3
```

Three exceptions keep things predictable:

- **Loop variables** belong to their loop. After `{% endfor %}` the name is
  gone, and assigning to it inside the loop changes only the current item.
- **`forloop`** exists only inside a loop.
- **A child template** rendered with `{% render %}` has its own variables. It
  cannot see the caller's, and its assignments do not leak back. See
  [Composition](/composition).

```vascula
{% assign total = 0 %}{% for price in settings.prices %}{% assign total = total | plus: price %}{% endfor %}Total: {{ total }}
```

```json data
{"settings": {"prices": [3, 4, 5]}}
```

```output
Total: 12
```

## Reserved names

`settings` and `forloop` cannot be assigned, captured, used as a loop variable
or passed to a child. Applications reserve their own globals the same way, so
a template cannot hide them.

```vascula
{% assign settings = "x" %}
```

```error
assignment name "settings" is reserved
```

Next: [Expressions and operators](/expressions).
