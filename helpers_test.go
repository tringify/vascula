package vascula

// siteGlobals binds site.settings the way a host does through Globals.
func siteGlobals(settings map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"site": map[string]interface{}{"settings": settings}}
}
