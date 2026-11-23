---
title: Composition
description: Render child templates with render, pass arguments, and how a child's scope and data work.
section: Language
order: 16
---

# Composition

`{% render "name" %}` renders another template, a **child**, in place.
Use it for anything you would otherwise copy and paste: a product card, a
price, an icon.

```vascula
<div class="grid">{% for p in settings.products %}{% render "card", product: p %}{% endfor %}</div>
```

```json data
{
  "settings": {"products": [{"title": "Cup"}, {"title": "Saucer"}]},
  "children": {"card": {"source": "<article>{{ product.title }}</article>"}}
}
```

```output
<div class="grid"><article>Cup</article><article>Saucer</article></div>
```

## Arguments

Everything after the name is `argument: value`. Each argument becomes a
variable in the child. Argument values are expressions, so filters work.

```vascula
{% render "price", currency: "EUR", amount: settings.cents | divided_by: 100 %}
```

```json data
{
  "settings": {"cents": 1250},
  "children": {"price": {"source": "{{ amount }} {{ currency }}"}}
}
```

```output
12.5 EUR
```

A filter's own arguments continue after a comma, so a filtered value
**must be the last argument**. Otherwise the filter takes the arguments that
follow as its own:

```vascula
{% render "price", amount: settings.cents | divided_by: 100, currency: "EUR" %}
```

```json data
{
  "settings": {"cents": 1250},
  "children": {"price": {"source": "{{ amount }} {{ currency }}"}}
}
```

```error
filter "divided_by" does not take a named argument "currency"
```

Put the filtered argument last, or `assign` it first and pass the variable.

An argument named `settings` or `forloop`, or a name the application
reserves, is a compilation error. Passing the same argument twice is too.

## A child has its own scope

A child starts fresh. It sees:

- its arguments,
- its **own** `settings`, supplied by the application when it resolves the child,
- the application's globals,
- the data roots the application allows **for the child**.

It does **not** see the caller's variables, `forloop`, `settings` or data
permissions. Whatever a child needs, pass it as an argument.

```vascula
{% assign secret = "caller only" %}{% render "child" %}
```

```json data
{"children": {"child": {"source": "{{ secret }}"}}}
```

```error
undeclared name "secret"
```

Nothing flows back either: a child's assignments stay in the child, and
`break` or `continue` in a child cannot end the caller's loop.

## Names are literal

The name after `render` must be a quoted string; a name computed from data is
a compilation error. [Inspection](/inspection) lists every template a template
renders.

```vascula
{% render settings.which %}
```

```error
requires a literal template name
```

## How children are found

The application supplies the children: it maps each name to a compiled
template, with that child's settings and data permissions (see
[Go API: children](/go-api#children)). An unknown name fails when the
`render` tag runs, with the application's reason in the message.

## Limits

A child shares its caller's render budget: work, output and value limits
cover the whole page, not each template. Children can render children, up to
32 levels deep, which stops a template that renders itself.

Next: [Output and escaping](/output).
