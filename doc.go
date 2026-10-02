// Package vascula compiles and renders Vascula templates.
//
// Templates read host-provided maps and slices, resolved settings and explicitly
// permitted context namespaces. A render has shared step, output and child-depth
// limits. Form destinations and extensions are supplied by the host.
//
// Text values are HTML-escaped by default. This is not an HTML sanitizer or
// contextual JavaScript, CSS or URL escaping system. A host accepting templates
// from untrusted authors must separately validate template markup, resources
// and allowed browser behavior. Host filters, callbacks and SafeHTML values are
// trusted capabilities. The host also owns authentication and form verification.
package vascula
