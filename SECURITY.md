# Security

Please report suspected vulnerabilities privately to **developers@tringify.com**.
Include the Vascula version, a minimal reproducer, the host options involved and the
impact. Do not attach live tokens, customer information or production templates.
We investigate every report and coordinate disclosure with you.

Security fixes go into the latest release.

## Trust boundary

Vascula escapes ordinary output as HTML text, checks the scheme of URL attributes
with printed parts, and bounds compilation and rendering. It does not sanitize
literal template HTML or perform contextual JavaScript or CSS escaping, and it
does not check URLs outside attribute values. A host must define who may author templates, validate browser
resources and choose which data and capabilities to expose.

Context allowances apply to entire roots, including dotted allowances. They are
not field-level authorization. Host filters, tags, resolvers, `SafeHTML` and
`HTMLAttr` are trusted code or values. Rendering limits are not a
process memory quota or an execution deadline for host callbacks.

Never render a template twice to evaluate data containing Vascula-looking text.
Keep source files outside public asset directories. Return a generic public error
on failure; retain diagnostics in an access-controlled development interface or
log. Engine errors can contain author-selected names and host-provided messages.
