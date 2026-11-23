# Changelog

## v1.1.0

- **Filters:** the `slug` filter makes a URL- and attribute-safe name from text.
- **Filters:** the `emphasize` filter escapes text and wraps the listed words
  in `<em>` (whole words, any case, any script).
- **Output observation:** a variable printed through a single `emphasize`
  filter is observed, with `OutputSpan.Markup` set.
- **Inspection:** `Template.FilterNames` lists every filter a template calls,
  with the position of its first call; `IsBuiltinFilter` and `BuiltinFilters`
  identify Vascula's own filters.

## v1.0.1

- **Repository:** a new README, issue and pull request templates, and a code of conduct. The language and the Go API are unchanged.

## v1.0.0

The first stable release of Vascula.

- **The template language:** variables, conditions, loops and ranges,
  reusable templates with `render`, and built-in filters for strings, HTML,
  numbers, arrays, dates, URLs and JSON.
- **Safe output:** everything a template prints is HTML-escaped, JSON is safe
  in a script and in an attribute, and a URL attribute with printed parts
  cannot carry a `javascript:` URL.
- **Explicit data:** a template reads only what your application gives it,
  and you can list every name and path it reads before you use it.
- **Bounded:** limits on template size and rendering work.
- **Clear errors** with a line and column.
- **A small Go library** with no third-party dependencies, plus your own
  filters, tags and forms, values computed only when read, and typed numbers
  and dates.
