package fnext

// good reading on this: https://twitter.com/sajma/status/757217773852487680
type contextKey string

// func (c contextKey) String() string {
// 	return "fnext context key " + string(c)
// }

// Keys for extensions to get things out of the context
var (
	// MiddlewareControllerKey is a context key. It can be used in handlers with context.WithValue to
	// access the MiddlewareContext.
	MiddlewareControllerKey = contextKey("middleware_controller")
	// AppNameKey
	AppNameKey = contextKey("app_name")
)
