---
title: Coming from Liquid
description: Every difference between Vascula and Liquid, for template authors and for people porting templates.
section: Guides
order: 40
---

# Coming from Liquid

Vascula's syntax is close to Liquid's, so most templates read the same. It is
its own language, stricter than Liquid in a few places. This page lists every
difference.

## Behavior

| | Liquid | Vascula |
|---|---|---|
| Output escaping | Off; use `escape` | Always on; no way to turn it off |
| `raw` filter, `{% raw %}` | Available | Not available |
| Reading an unknown name | Prints nothing | Error, unless the application allows that root |
| Unknown filter | Ignored or error, depending on mode | Error |
| `"5" > 3` | Compares somehow | Error: ordering needs two numbers or two strings |
| `"5" == 5` | False | True: a numeric string equals its number |
| `10 \| divided_by: 4` | `2` (integer division) | `2.5` |
| Integers | Arbitrary size | Exact 64-bit integers |
| Variables assigned in a block | Global | Global (the same) |
| `include` | Shares the caller's variables | Removed; `render` only, with its own scope |
| `render` with a variable name | Not allowed | Not allowed (the same) |
| `and` / `or` | Evaluate right to left | Evaluate left to right, short-circuit, and return an operand |
| JSON inside an attribute | Breaks the attribute | Escaped for the attribute automatically |
| Errors | Often rendered inline | Returned to the application with line and column |
| Resource use | Depends on the host | Bounded: work, output, value size, depth |

## Tags

| Liquid tag | Vascula |
|---|---|
| `if`, `elsif`, `else`, `unless` | Same. `unless` has no `elsif`. |
| `case`, `when` | Same, including `when a, b` and `when a or b`. |
| `for` with `limit`, `offset`, `reversed`, `else` | Same. Ranges `(1..n)` work. |
| `forloop.*` | Same properties, including `parentloop`. |
| `break`, `continue`, `cycle` | Same. Cycle groups (`cycle "g": ...`) are not supported. |
| `assign`, `capture` | Same. |
| `comment`, `{% # %}` | `{% comment %}` and `{# ... #}`. |
| `increment`, `decrement` | Not supported; use `assign` with `plus`. |
| `tablerow` | Not supported; use `for` with `forloop`. |
| `liquid`, `echo` | Not supported. |
| `render` | Same idea; arguments with `name: value`. `with` and `for` forms are not supported. |
| `include` | Not supported. |
| `raw` | Not supported. |
| Shopify `form`, `section`, `schema`, `style`, `javascript`, `paginate` | Not part of the language; an application can add its own tags. |

## Filters

These Liquid filters exist in Vascula with the same meaning: `abs`, `append`,
`at_least`, `at_most`, `capitalize`, `ceil`, `compact`, `concat`, `date`,
`default`, `divided_by` (but see the table above), `downcase`, `escape`,
`escape_once`, `first`, `floor`, `join`, `last`, `lstrip`, `map`, `minus`,
`modulo`, `newline_to_br`, `plus`, `prepend`, `remove`, `remove_first`,
`replace`, `replace_first`, `reverse`, `round`, `rstrip`, `size`, `slice`,
`sort`, `sort_natural`, `split`, `strip`, `strip_html`, `strip_newlines`,
`sum`, `times`, `truncate`, `truncatewords`, `uniq`, `upcase`, `url_decode`,
`url_encode`, `where`.

Vascula adds `json`.

Not available: `raw`, `base64_*`,
`url_escape`, `remove_last`, `replace_last`, `find`, `find_index`, `has`,
`reject`, `group_by`, and Shopify's commerce filters (`money`, `img_url`, `t`
and others). Applications provide their own filters for formatting money,
building URLs and translating text.

## Porting a template

1. Compile it. Every unsupported tag or syntax error is reported with its line
   and column.
2. Remove `| raw` and `{% raw %}`. If something must print HTML, the
   application has to pass it as trusted HTML.
3. Replace `include` with `render`, passing what the child needs as arguments.
4. Render it with the data it needs and fix each `undeclared name` error:
   either it is a typo, or the application must allow that root.
5. Check divisions: unlike Liquid, `divided_by` does not truncate integer
   division; add `| floor` where you relied on that.
