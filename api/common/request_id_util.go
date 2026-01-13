package common

import (
	"context"

	"github.com/fnproject/fn/api/id"
)

// FnRequestID returns the passed value if that is not empty otherwise it generates a new unique ID
func FnRequestID(ridFound string) string {
	if ridFound == "" {
		return id.New().String()
	}
	return ridFound
}

// RequestIDFromContext extract the request id from the context
func RequestIDFromContext(ctx context.Context) string {
	rid, _ := ctx.Value(contextKey(RequestIDContextKey)).(string)
	return rid
}
