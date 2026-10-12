# Changelog

## v0.1.5

- **URL attributes:** when anything printed stands in a URL attribute (`href`, `src`, `srcset`, `action`, `formaction`, `poster`, `data`, `cite`, `background`, `ping`, `manifest`, `xlink:href`, and names containing `src`, `uri` or `url`) or a `style` attribute, the whole value, with its literal text and character references decoded, must be relative or use `http`, `https`, `mailto` or `tel`; anything else, such as `javascript:` or `data:text/html`, now prints as `#`. `srcset` is read candidate by candidate as browsers read it, and each `url()` in `style` is checked. The `src` of `<img>` and `<source>` and `url()` also accept `data:image/` URLs.
- **Unquoted attribute values:** output in an unquoted value escapes spaces, quotes, `` ` ``, `=`, `/`, `<` and `>`, so it cannot end the value.
- **Output where an attribute goes:** the new `HTMLAttr` type is attribute markup the host vouches for (such as a filter returning `loading="lazy"`) and prints as written; any other value, including `SafeHTML`, must be one plain attribute name (not an event handler, `style` or URL attribute) and otherwise prints nothing.
- **`SafeHTML` inside a tag (breaking):** in an attribute value, `SafeHTML` has its tags removed and the rest escaped, as in Go's `html/template`; it prints unescaped only in the page body. Hosts that returned attribute markup as `SafeHTML` must return `HTMLAttr`.
- **Compile errors (breaking):** a template no longer compiles when it prints an element name or part of an attribute name, prints two values in a tag with no space between them, uses `render` inside a tag other than in a quoted value, ends (or ends a `capture`) inside a tag, or when `if`/`unless`/`case` branches, a loop body, or `break`/`continue` leave the markup in different places, as in Go's `html/template`. Control flow cannot split a name: text or output continuing an element or attribute name that a branch may end inside (`<a {% if c %}h{% endif %}ref="…">`) fails to compile, as does a loop body or `break`/`continue` ending inside a name or a loop starting inside one.
- **Dates:** an `Instant` compares with another date, or with an ISO 8601 string (`"2026-10-01"`, `"2026-10-01T09:30:00Z"`), by the moment it names, for both equality and ordering.
- **Host numbers and dates:** a value implementing `Number` (`Float64` and `String`, like `json.Number`) is a number in comparisons, arithmetic and number filters and prints as its `String`; a value implementing `Instant` (`Time` and `String`) works with the `date` filter and compares with other dates, and prints as its `String`.

## v0.1.4

- **Lazy values:** a `*Lazy` (`NewLazy`) in data, settings or globals is computed the first time a render uses it, through every access path: reads, loops, output, filters, comparisons and host code arguments. `Resolve` computes the `Lazy` values inside a value for host code.

## v0.1.3

- Output observation also identifies a variable printed through a single `default` filter when its own value, not the fallback, is printed.

## v0.1.2

- Optional host output observation identifies direct variable outputs in HTML text without changing rendered bytes. Attribute, raw-text, captured, filtered and child outputs are excluded.

## v0.1.1

- **Documentation:** corrected the number of built-in filters.

## v0.1.0

The first public release of Vascula.

- **The template language:** variables, conditions, loops and ranges,
  reusable templates with `render`, and 50 built-in filters.
- **Safe output:** everything a template prints is HTML-escaped.
- **Explicit data:** a template reads only what your application gives it,
  and you can list every name it reads before you use it.
- **Bounded:** limits on template size and rendering work.
- **Clear errors** with a line and column.
- **A small Go library** with no third-party dependencies, plus your own
  filters, tags and forms.
