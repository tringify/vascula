// Package vascula compiles and renders Vascula templates.
//
// Templates read host-provided maps and slices, resolved settings and explicitly
// permitted context namespaces. A render has shared step, output and child-depth
// limits. Form destinations and extensions are supplied by the host.
//
// Text values are HTML-escaped by default, and a URL attribute with printed
// parts must be relative or use http, https, mailto or tel. This is not an HTML
// sanitizer or a contextual JavaScript or CSS escaping system. A host accepting templates
// from untrusted authors must separately validate template markup, resources
// and allowed browser behavior. Host filters, tags, SafeHTML and HTMLAttr values
// are trusted capabilities. The host also owns authentication and form verification.
package vascula
