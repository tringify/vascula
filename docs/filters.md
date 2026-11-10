---
title: Filters
description: Every built-in filter, with its arguments, exact behavior and edge cases.
section: Language
order: 18
---

# Filters

A filter transforms the value on its left: `{{ value | filter: argument }}`.
See [Expressions](/expressions#filters) for how chaining and arguments work.

Filters convert their input as needed: string filters treat a number as its
printed text, number filters read a numeric string as a number, and array
filters treat `nil` as an empty array. A filter that cannot work with its
arguments fails with an error that names it.

Applications add their own filters. Three built-ins can never be replaced by
an application: `escape`, `json` and `default`, because they define how output
stays safe.

**On this page:** [Strings](#strings) · [HTML](#html) · [Numbers](#numbers) ·
[Arrays](#arrays) · [Dates](#dates) · [URLs](#urls) · [Fallbacks](#fallbacks) ·
[JSON](#json)

## Strings

### upcase

Converts to upper case.

```vascula
{{ "Café au lait" | upcase }}
```

```output
CAFÉ AU LAIT
```

### downcase

Converts to lower case.

```vascula
{{ "Café AU Lait" | downcase }}
```

```output
café au lait
```

### capitalize

Upper-cases the first character and lower-cases the rest.

```vascula
{{ "éCLAIR WITH CREAM" | capitalize }}
```

```output
Éclair with cream
```

### strip, lstrip, rstrip

Remove whitespace from both ends, the start, or the end.

```vascula
[{{ "  tea  " | strip }}] [{{ "  tea  " | lstrip }}] [{{ "  tea  " | rstrip }}]
```

```output
[tea] [tea  ] [  tea]
```

### strip_newlines

Removes every line break.

```vascula
{{ settings.text | strip_newlines }}
```

```json data
{"settings": {"text": "one\ntwo\r\nthree"}}
```

```output
onetwothree
```

### append, prepend

`append: text` adds text at the end; `prepend: text` adds it at the start.

```vascula
{{ "tea" | append: "pot" }} {{ "pot" | prepend: "tea" }}
```

```output
teapot teapot
```

### replace, replace_first

`replace: old, new` replaces every occurrence; `replace_first` only the first.

```vascula
{{ "a-b-c" | replace: "-", "+" }} {{ "a-b-c" | replace_first: "-", "+" }}
```

```output
a+b+c a+b-c
```

### remove, remove_first

`remove: text` deletes every occurrence; `remove_first` only the first.

```vascula
{{ "a-b-c" | remove: "-" }} {{ "a-b-c" | remove_first: "-" }}
```

```output
abc ab-c
```

### truncate

`truncate: length, ending` shortens text to at most `length` characters,
**counting the ending**. The default length is 50 and the default ending
is `...`. Text that already fits is unchanged. Characters are never split.

```vascula
{{ "Vascula templates" | truncate: 10 }} | {{ "Vascula templates" | truncate: 10, "…" }} | {{ "Short" | truncate: 10 }}
```

```output
Vascula... | Vascula t… | Short
```

### truncatewords

`truncatewords: count, ending` keeps the first `count` words and adds the
ending (default `...`) when it removed any. Runs of whitespace become single
spaces. The count must be at least 1.

```vascula
{{ "one two  three four" | truncatewords: 2 }} | {{ "one two" | truncatewords: 5 }}
```

```output
one two... | one two
```

### split

`split: separator` splits text into an array. An empty separator splits into
single characters; an empty input gives an empty array.

```vascula
{{ "a,b,,c" | split: "," | join: "|" }} {{ "abc" | split: "" | join: "." }} {{ "" | split: "," | size }}
```

```output
a|b||c a.b.c 0
```

### slice

`slice: offset, length` returns part of a string or an array. A negative
offset counts from the end. The length defaults to 1.

```vascula
{{ "Vascula" | slice: 0, 3 }} {{ "Vascula" | slice: -3 }} {{ "Vascula" | slice: -3, 3 }} {{ settings.list | slice: 1, 2 | join: "," }}
```

```json data
{"settings": {"list": ["a", "b", "c", "d"]}}
```

```output
Vas u ula b,c
```

### size

The number of characters in a string, items in an array, or keys in a map.
Anything else is 0. The `.size` property gives the same result.

```vascula
{{ "Café" | size }} {{ settings.list | size }} {{ nil | size }}
```

```json data
{"settings": {"list": [1, 2, 3]}}
```

```output
4 3 0
```

## HTML

### escape

Escapes text for HTML. Printing the result does not escape it again, so
`escape` never double-escapes. Output is escaped anyway; use `escape` when you
store markup-safe text in a variable or pass it to an application filter.

```vascula
{{ "<b>Tea</b> & cake" | escape }}
```

```output
&lt;b&gt;Tea&lt;/b&gt; &amp; cake
```

### escape_once

Escapes text that may already contain character references, without escaping
them a second time.

```vascula
{{ "1 &lt; 2 & 3 > 2" | escape_once }}
```

```output
1 &lt; 2 &amp; 3 &gt; 2
```

### strip_html

Removes everything between `<` and `>`. The result is text, and is escaped
when printed. This is not a sanitizer: use it to make a plain-text summary,
not to make markup safe.

```vascula
{{ "<p>Hello <b>world</b></p>" | strip_html }}
```

```output
Hello world
```

### newline_to_br

Escapes the text and replaces each line break with `<br>`.

```vascula
{{ settings.text | newline_to_br }}
```

```json data
{"settings": {"text": "line one\n<line two>"}}
```

```output
line one<br>&lt;line two&gt;
```

## Numbers

Number filters read numbers and numeric strings. Two integers give an exact
integer result (falling back to a decimal on 64-bit overflow); anything with a
decimal gives a decimal.

### plus, minus, times

```vascula
{{ 7 | plus: 3 }} {{ 7 | minus: 10 }} {{ 7 | times: 3 }} {{ 1.5 | times: 2 }} {{ "4" | plus: 1 }}
```

```output
10 -3 21 3 5
```

### divided_by

Divides. Two integers that divide exactly give an integer; otherwise the
result is a decimal. Dividing by zero is an error.

```vascula
{{ 10 | divided_by: 2 }} {{ 7 | divided_by: 2 }} {{ 1 | divided_by: 3 | round: 4 }}
```

```output
5 3.5 0.3333
```

```vascula
{{ 1 | divided_by: 0 }}
```

```error
divided_by: division by zero
```

### modulo

The remainder of a division, exact for two integers. Its sign follows the
input. A zero divisor is an error.

```vascula
{{ 7 | modulo: 3 }} {{ -7 | modulo: 3 }} {{ 7.5 | modulo: 2 }}
```

```output
1 -1 1.5
```

### round, ceil, floor

Round to the nearest integer, up, or down. An optional argument keeps that
many decimal places. Halves round away from zero.

```vascula
{{ 2.5 | round }} {{ 3.14159 | round: 2 }} {{ 2.1 | ceil }} {{ 2.9 | floor }} {{ -2.5 | round }}
```

```output
3 3.14 3 2 -3
```

### abs

The absolute value.

```vascula
{{ -7 | abs }} {{ 2.5 | abs }}
```

```output
7 2.5
```

### at_least, at_most

`at_least: n` returns the larger of the value and `n`; `at_most: n` the
smaller. Use them to clamp.

```vascula
{{ 3 | at_least: 5 }} {{ 30 | at_most: 12 }} {{ 8 | at_least: 1 | at_most: 10 }}
```

```output
5 12 8
```

## Arrays

### join

`join: separator` prints each item and joins them. The separator defaults to
no separator.

```vascula
{{ settings.list | join: ", " }} {{ settings.list | join }}
```

```json data
{"settings": {"list": ["a", "b", "c"]}}
```

```output
a, b, c abc
```

### first, last

The first or last item, or `nil` for an empty array. The `.first` and `.last`
properties do the same.

```vascula
{{ settings.list | first }} {{ settings.list | last }}
```

```json data
{"settings": {"list": ["a", "b", "c"]}}
```

```output
a c
```

### map

`map: field` returns an array of one field from each item.

```vascula
{{ settings.people | map: "name" | join: ", " }}
```

```json data
{"settings": {"people": [{"name": "Ada"}, {"name": "Grace"}]}}
```

```output
Ada, Grace
```

### where

`where: field` keeps the items whose field is truthy. `where: field, value`
keeps the items whose field equals the value.

```vascula
{{ settings.items | where: "available" | map: "name" | join: "," }} / {{ settings.items | where: "color", "red" | map: "name" | join: "," }}
```

```json data
{"settings": {"items": [
  {"name": "cup", "available": true, "color": "red"},
  {"name": "pot", "available": false, "color": "red"},
  {"name": "jug", "available": true, "color": "blue"}
]}}
```

```output
cup,jug / cup,pot
```

### sort, sort_natural

`sort` orders items ascending; `sort: field` orders by a field. Numbers sort
as numbers and strings in character order (upper case before lower case).
`sort_natural` compares the printed text case-insensitively. Items that cannot
be compared keep their original order.

```vascula
{{ settings.nums | sort | join: "," }} {{ settings.words | sort | join: "," }} {{ settings.words | sort_natural | join: "," }} {{ settings.items | sort: "price" | map: "name" | join: "," }}
```

```json data
{"settings": {
  "nums": [10, 2, 33],
  "words": ["banana", "Cherry", "apple"],
  "items": [{"name": "b", "price": 5}, {"name": "a", "price": 2}]
}}
```

```output
2,10,33 Cherry,apple,banana apple,banana,Cherry a,b
```

### reverse

Reverses the order.

```vascula
{{ (1..4) | reverse | join: "," }}
```

```output
4,3,2,1
```

### uniq

Removes repeated items, keeping the first of each. Items compare with
[equality](/values#comparing-values), so `1` and `1.0` are the same.

```vascula
{{ settings.list | uniq | join: "," }}
```

```json data
{"settings": {"list": ["a", "b", "a", "c", "b"]}}
```

```output
a,b,c
```

### compact

Removes `nil` items.

```vascula
{{ settings.list | compact | join: "," }}
```

```json data
{"settings": {"list": ["a", null, "b", null]}}
```

```output
a,b
```

### concat

`concat: other` joins two arrays.

```vascula
{{ settings.a | concat: settings.b | join: "," }}
```

```json data
{"settings": {"a": [1, 2], "b": [3]}}
```

```output
1,2,3
```

### sum

Adds up the items, or `sum: field` adds up one field of each item. Items that
are not numbers are skipped.

```vascula
{{ settings.prices | sum }} {{ settings.lines | sum: "qty" }}
```

```json data
{"settings": {"prices": [1.5, 2, "x"], "lines": [{"qty": 2}, {"qty": 3}]}}
```

```output
3.5 5
```

## Dates

### date

`date: format` formats a date. The input is a date string
(`2026-10-02`, `2026-10-02 14:30:00`, or RFC 3339 such as
`2026-10-02T14:30:00Z`), a date value from the application, or `"now"` or
`"today"` (the current time in UTC). The format uses strftime directives:

| Directive | Meaning | Example |
|---|---|---|
| `%Y`, `%y` | year, two-digit year | `2026`, `26` |
| `%m`, `%d`, `%e` | month, day, day padded with a space | `10`, `02`, ` 2` |
| `%B`, `%b` | month name, short month name | `October`, `Oct` |
| `%A`, `%a` | weekday, short weekday | `Friday`, `Fri` |
| `%H`, `%I`, `%p` | hour (24h), hour (12h), AM/PM | `14`, `02`, `PM` |
| `%M`, `%S` | minute, second | `30`, `00` |
| `%j` | day of the year | `275` |
| `%s` | Unix seconds | `1790951400` |
| `%Z`, `%z` | time zone name, offset | `UTC`, `+0000` |
| `%%` | a percent sign | `%` |

Other directives are printed as written. A value that is not a date, or a
missing format, is an error.

```vascula
{{ "2026-10-02T14:30:00Z" | date: "%A, %e %B %Y at %H:%M" }} | {{ "2026-10-02" | date: "%d/%m/%y" }}
```

```output
Friday,  2 October 2026 at 14:30 | 02/10/26
```

```vascula
{{ "next week" | date: "%Y" }}
```

```error
date: cannot parse "next week" as a date
```

## URLs

### url_encode, url_decode

Encode text for use in a URL query, or decode it. Spaces become `+`.

```vascula
{{ "tea & cake" | url_encode }} {{ "tea+%26+cake" | url_decode }}
```

```output
tea+%26+cake tea &amp; cake
```

## Fallbacks

### default

`default: fallback` returns the fallback when the value is `nil`, `false`, an
empty string or an empty collection. Unlike a condition, it treats `""` as
missing. `0` is a real value and is kept.

With `allow_false: true`, an explicit `false` is kept, which lets a setting
that defaults to `true` still be switched off.

```vascula
[{{ settings.title | default: "Untitled" }}] [{{ settings.count | default: 10 }}] [{{ settings.missing | default: "none" }}] [{{ settings.flag | default: true }}] [{{ settings.flag | default: true, allow_false: true }}]
```

```json data
{"settings": {"title": "", "count": 0, "flag": false}}
```

```output
[Untitled] [0] [none] [true] [false]
```

## JSON

### json

Converts a value to JSON. Inside `<script>` it is printed as JSON with `<`,
`>` and `&` encoded; inside an HTML tag it is also escaped for HTML. See
[Output: JSON](/output#json).

```vascula
<script>const data = {{ settings.data | json }};</script>
```

```json data
{"settings": {"data": {"tags": ["a", "<b>"], "count": 2}}}
```

```output
<script>const data = {"count":2,"tags":["a","\u003cb\u003e"]};</script>
```

Map keys are written in sorted order.
