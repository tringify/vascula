---
title: Loops
description: for loops, limit/offset/reversed, ranges, forloop, break, continue, empty collections and cycle.
section: Language
order: 15
---

# Loops

## for

`for` renders its body once for each item of an array.

```vascula
<ul>{% for tag in product.tags %}<li>{{ tag }}</li>{% endfor %}</ul>
```

```json data
{"data": {"product": {"tags": ["new", "sale"]}}, "allow": ["product"]}
```

```output
<ul><li>new</li><li>sale</li></ul>
```

The collection is a single value: a variable path or a range. It cannot take
filters; assign a filtered list to a variable first.

```vascula
{% assign sorted = settings.names | sort %}{% for name in sorted %}{{ name }} {% endfor %}
```

```json data
{"settings": {"names": ["Grace", "Ada"]}}
```

```output
Ada Grace 
```

### An empty collection: else

An `else` inside a loop renders when there is nothing to loop over.

```vascula
{% for item in settings.items %}{{ item }}{% else %}Your basket is empty.{% endfor %}
```

```json data
{"settings": {"items": []}}
```

```output
Your basket is empty.
```

### What can be looped over

| Value | Iterations |
|---|---|
| Array | each item, in order |
| Range `(a..b)` | each integer |
| Map | `[key, value]` pairs, sorted by key; read `item[0]` and `item[1]` |
| `nil`, missing | none (renders `else`) |
| String, number, boolean | none (renders `else`) |

```vascula
{% for pair in settings.sizes %}{{ pair[0] }}={{ pair[1] }} {% endfor %}
```

```json data
{"settings": {"sizes": {"m": 10, "l": 4, "s": 0}}}
```

```output
l=4 m=10 s=0 
```

## limit, offset, reversed

Modifiers come after the collection, in any order: `offset` skips items,
`limit` caps how many remain, and `reversed` reverses the selection.

```vascula
{% for n in (1..10) offset: 2 limit: 3 %}{{ n }}{% endfor %} {% for n in (1..10) limit: 3 reversed %}{{ n }}{% endfor %}
```

```output
345 321
```

`offset` is applied first, then `limit`, then `reversed`. A negative offset is
treated as zero; a negative limit means no limit. Both accept a variable or a
filter expression: `limit: settings.count | plus: 1`.

## Ranges

A range loops over integers. Each end is a number or a variable.

```vascula
{% for i in (1..settings.stars) %}★{% endfor %}{% for i in (settings.stars..4) %}☆{% endfor %}
```

```json data
{"settings": {"stars": 3}}
```

```output
★★★☆☆
```

Ranges are limited in size like any other value; see [limits](/safety).

## forloop

Inside a loop, `forloop` describes the current iteration.

| Property | Value |
|---|---|
| `forloop.index` | position, starting at 1 |
| `forloop.index0` | position, starting at 0 |
| `forloop.rindex` | positions remaining, ending at 1 |
| `forloop.rindex0` | positions remaining, ending at 0 |
| `forloop.first` | `true` on the first iteration |
| `forloop.last` | `true` on the last iteration |
| `forloop.length` | number of iterations (after `offset`, `limit`) |
| `forloop.parentloop` | the enclosing loop's `forloop`, or `nil` |

```vascula
{% for t in settings.tags %}{{ t }}{% unless forloop.last %}, {% endunless %}{% endfor %} ({{ settings.tags.size }})
```

```json data
{"settings": {"tags": ["red", "green", "blue"]}}
```

```output
red, green, blue (3)
```

In a nested loop, `forloop.parentloop` reaches the outer loop:

```vascula
{% for row in (1..2) %}{% for col in (1..3) %}{{ forloop.parentloop.index }}{{ forloop.index }} {% endfor %}{% endfor %}
```

```output
11 12 13 21 22 23 
```

## break and continue

`break` stops the loop; `continue` skips to the next item. They act on the
innermost loop. Using them outside a loop is an error.

```vascula
{% for n in (1..10) %}{% if n == 3 %}{% continue %}{% endif %}{% if n > 5 %}{% break %}{% endif %}{{ n }}{% endfor %}
```

```output
1245
```

## cycle

`cycle` prints the next of its values each time it runs, starting over after
the last. Each `cycle` tag keeps its own position.

```vascula
{% for n in (1..5) %}<tr class="{% cycle "odd", "even" %}">{% endfor %}
```

```output
<tr class="odd"><tr class="even"><tr class="odd"><tr class="even"><tr class="odd">
```

## Loop variables

The loop variable exists only inside its loop. Variables you `assign` inside a
loop remain after it; see [Scope](/variables#scope).

Next: [Composition](/composition).
