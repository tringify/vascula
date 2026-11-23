---
title: Tutorial
description: Build a product list step by step and learn the language along the way.
section: Start
order: 3
---

# Tutorial: a product list

In this tutorial you build the product list of a small tea shop, one feature
at a time. Each step shows the template, the data it is rendered with, and
the output. You need nothing installed to follow along.

The data is the same throughout. The application gives the template one data
root, `shop`, and two settings:

```json
{
  "data": {"shop": {
    "name": "Leaf & Pot",
    "products": [
      {"title": "Sencha", "price": 1250, "stock": 12, "tags": ["green", "japan"]},
      {"title": "Assam <strong>", "price": 899, "stock": 0, "tags": ["black"]},
      {"title": "Oolong", "price": 1575, "stock": 3, "tags": []}
    ]
  }},
  "allow": ["shop"],
  "settings": {"heading": "Our teas", "show_tags": true}
}
```

## 1. Print a value

`{{ }}` prints a value. `shop.name` reads the field `name` of the root `shop`;
`settings.heading` reads a setting.

```vascula
<h1>{{ settings.heading }} at {{ shop.name }}</h1>
```

```json data
{"data": {"shop": {"name": "Leaf & Pot"}}, "allow": ["shop"], "settings": {"heading": "Our teas"}}
```

```output
<h1>Our teas at Leaf &amp; Pot</h1>
```

The `&` was escaped automatically. Everything you print is.

## 2. Loop over the products

`{% for %}` repeats its body for each item. Inside, `product` is the current
item.

```vascula
<ul>{% for product in shop.products %}
  <li>{{ product.title }}</li>{% endfor %}
</ul>
```

```json data
{"data": {"shop": {"products": [{"title": "Sencha"}, {"title": "Assam <strong>"}, {"title": "Oolong"}]}}, "allow": ["shop"]}
```

```output
<ul>
  <li>Sencha</li>
  <li>Assam &lt;strong&gt;</li>
  <li>Oolong</li>
</ul>
```

A product title containing markup cannot break the page: it is printed as
text.

## 3. Format the price with filters

Prices are in cents. Filters, written after `|`, transform a value, and you
can chain them: divide by 100, then round to two decimal places. Notice that
`12.5` prints without a trailing zero: proper money formatting is a job for a
`money` filter that the application provides (see
[Extending](/extending#filters)).

```vascula
{% for product in shop.products %}{{ product.title }}: €{{ product.price | divided_by: 100.0 | round: 2 }}
{% endfor %}
```

```json data
{"data": {"shop": {"products": [{"title": "Sencha", "price": 1250}, {"title": "Oolong", "price": 1575}]}}, "allow": ["shop"]}
```

```output
Sencha: €12.5
Oolong: €15.75
```

## 4. Show stock with conditions

`{% if %}` chooses what to show. Conditions compare values with `==`, `>`
and the other operators.

```vascula
{% for product in shop.products %}{{ product.title }}: {% if product.stock > 5 %}in stock{% elsif product.stock > 0 %}only {{ product.stock }} left{% else %}sold out{% endif %}
{% endfor %}
```

```json data
{"data": {"shop": {"products": [{"title": "Sencha", "stock": 12}, {"title": "Assam", "stock": 0}, {"title": "Oolong", "stock": 3}]}}, "allow": ["shop"]}
```

```output
Sencha: in stock
Assam: sold out
Oolong: only 3 left
```

## 5. List the tags, or nothing

`join` turns an array into text. An empty array is still *truthy*, so test its
size rather than the array itself. The setting `show_tags` lets the shop
owner switch tags off.

```vascula
{% for product in shop.products %}{{ product.title }}{% if settings.show_tags and product.tags.size > 0 %} ({{ product.tags | join: ", " }}){% endif %}
{% endfor %}
```

```json data
{"data": {"shop": {"products": [{"title": "Sencha", "tags": ["green", "japan"]}, {"title": "Oolong", "tags": []}]}}, "allow": ["shop"], "settings": {"show_tags": true}}
```

```output
Sencha (green, japan)
Oolong
```

## 6. Count with variables

`assign` stores a value. Count the products in stock as you go, and print the
total after the loop.

```vascula
{% assign available = 0 %}{% for product in shop.products %}{% if product.stock > 0 %}{% assign available = available | plus: 1 %}{% endif %}{% endfor %}{{ available }} of {{ shop.products.size }} teas available
```

```json data
{"data": {"shop": {"products": [{"stock": 12}, {"stock": 0}, {"stock": 3}]}}, "allow": ["shop"]}
```

```output
2 of 3 teas available
```

The same in one line, with filters: `{{ shop.products | where: "stock" | size }}`
would not work here, because a stock of `0` is truthy. Variables are clearer.

## 7. Move the card into its own template

The product markup is getting long. `{% render %}` renders another template,
and passes it what it needs as arguments. The child sees only its arguments,
so it cannot accidentally depend on the page around it.

```vascula
<ul>{% for product in shop.products %}{% render "product-card", product: product %}{% endfor %}</ul>
```

```json data
{
  "data": {"shop": {"products": [{"title": "Sencha", "stock": 12}, {"title": "Assam", "stock": 0}]}},
  "allow": ["shop"],
  "children": {"product-card": {"source": "<li class=\"{% if product.stock == 0 %}sold-out{% endif %}\">{{ product.title }}</li>"}}
}
```

```output
<ul><li class="">Sencha</li><li class="sold-out">Assam</li></ul>
```

## 8. Put it together

```vascula
<h1>{{ settings.heading }} at {{ shop.name }}</h1>
{%- assign available = 0 -%}
{%- for product in shop.products -%}
  {%- if product.stock > 0 %}{% assign available = available | plus: 1 %}{% endif -%}
{%- endfor %}
<p>{{ available }} of {{ shop.products.size }} teas available</p>
<ul>
{%- for product in shop.products %}
  <li>{{ product.title }}: €{{ product.price | divided_by: 100.0 | round: 2 }}
  {%- if product.stock == 0 %} (sold out){% endif -%}
  {%- if settings.show_tags and product.tags.size > 0 %} [{{ product.tags | join: ", " }}]{% endif -%}
  </li>
{%- endfor %}
</ul>
```

```json data
{
  "data": {"shop": {
    "name": "Leaf & Pot",
    "products": [
      {"title": "Sencha", "price": 1250, "stock": 12, "tags": ["green", "japan"]},
      {"title": "Assam <strong>", "price": 899, "stock": 0, "tags": ["black"]},
      {"title": "Oolong", "price": 1575, "stock": 3, "tags": []}
    ]
  }},
  "allow": ["shop"],
  "settings": {"heading": "Our teas", "show_tags": true}
}
```

```output
<h1>Our teas at Leaf &amp; Pot</h1>
<p>2 of 3 teas available</p>
<ul>
  <li>Sencha: €12.5 [green, japan]</li>
  <li>Assam &lt;strong&gt;: €8.99 (sold out) [black]</li>
  <li>Oolong: €15.75</li>
</ul>
```

The `-` inside `{%-` and `-%}` removes the whitespace around those tags, which
keeps the output tidy without cramming the template onto one line. See
[Whitespace control](/syntax#whitespace-control).

## Where next

- The rest of the language: [Variables](/variables), [Loops](/loops),
  [Filters](/filters).
- Rendering this from Go: [Getting started](/getting-started).
