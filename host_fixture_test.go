package vascula

// These are examples for language tests.
// Production hosts must supply their own canonical form definitions.
func compileFixture(src string) (*Template, error) {
	forms := map[string]FormDefinition{
		"save":   {Path: "/save", TokenContextKey: "token", TokenField: "authenticity_token", ActionAttribute: "data-action"},
		"upload": {Path: "/upload", Multipart: true, TokenContextKey: "token", TokenField: "authenticity_token", ActionAttribute: "data-action"},
		"login":  {Path: "/login", TokenContextKey: "token", TokenField: "authenticity_token", ActionAttribute: "data-action"},
	}
	return CompileWithOptions(src, CompileOptions{Forms: forms})
}
