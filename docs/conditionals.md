---
title: Conditionals
description: if, elsif, else, unless and case/when.
section: Language
order: 14
---

# Conditionals

## if, elsif, else

`if` renders its body when the condition is [truthy](/values#truthiness).
`elsif` adds more conditions, tried in order; `else` renders when none matched.

```vascula
{% if settings.stock > 10 %}In stock{% elsif settings.stock > 0 %}Only {{ settings.stock }} left{% else %}Sold out{% endif %}
```

```json data
{"settings": {"stock": 3}}
```

```output
Only 3 left
```

Remember that `0` and `""` are truthy. To test for "has something", compare
with `blank` or check the size:

```vascula
{% if settings.title != blank %}<h2>{{ settings.title }}</h2>{% endif %}{% if settings.items.size > 0 %}{{ settings.items.size }} items{% endif %}
```

```json data
{"settings": {"title": "", "items": ["a"]}}
```

```output
1 items
```

## unless

`unless` is `if` with the condition reversed: its body renders when the
condition is **not** truthy. It may have an `else`, but no `elsif`.

```vascula
{% unless settings.signed_in %}<a href="/login">Sign in</a>{% else %}Welcome back{% endunless %}
```

```json data
{"settings": {"signed_in": false}}
```

```output
<a href="/login">Sign in</a>
```

## case, when

`case` compares one value against several, using [equality](/values#comparing-values).
The first matching `when` renders. A `when` can list several values, separated
by commas or `or`. `else` renders when nothing matched.

```vascula
{% case settings.size %}{% when "s", "xs" %}Small{% when "m" or "l" %}Regular{% else %}Large{% endcase %}
```

```json data
{"settings": {"size": "l"}}
```

```output
Regular
```

Numbers compare as numbers, so `{% when 2 %}` matches `2.0`.

```vascula
{% case settings.qty %}{% when 0 %}none{% when 1 %}one{% else %}many{% endcase %}
```

```json data
{"settings": {"qty": 1.0}}
```

```output
one
```

A `case` needs at least one `when`. Anything written between `{% case %}` and
the first `{% when %}` is ignored, so you can lay the tags out on separate
lines.

Next: [Loops](/loops).
