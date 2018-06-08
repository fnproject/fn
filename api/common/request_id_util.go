package common

import (
	"context"

	"github.com/fnproject/fn/api/id"
)

const (
	maxLength = 32
)

// FnRequestID returns the passed value if that is not empty or too long otherwise it generates a new unique ID
func FnRequestID(ridFound string) string {
	if ridFound == "" {
		return id.New().String()
	}
	if len(ridFound) > maxLength {
		// we truncate the rid to the value specified by maxLenght const
		return ridFound[:maxLength]
	}
	return ridFound
}

//IncomingRID extract the request id from ctx
func IncomingRID(ctx context.Context) string {
	rid, found := ctx.Value(RIDContextKey()).(string)
	if !found {
		log := Logger(ctx)
		log.Debug("Unable to find fn request ID in the context")
	}
	return rid
}
