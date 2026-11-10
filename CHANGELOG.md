# Changelog

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
