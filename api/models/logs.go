package models

import (
	"context"
	"io"
)

type LogStore interface {
	// InsertLog will insert the log at callID, overwriting if it previously
	// existed.
	InsertLog(ctx context.Context, appID, callID string, callLog io.Reader) error

	// GetLog will return the log at callID, an error will be returned if the log
	// cannot be found.
	GetLog(ctx context.Context, appID, callID string) (io.Reader, error)

	// TODO we should probably allow deletion of a range of logs (also calls)?
	// common cases for deletion will be:
	// * route gets nuked
	// * app gets nuked
	// * call+logs getting cleaned up periodically

	// InsertCall inserts a call into the datastore, it will error if the call already
	// exists.
	InsertCall(ctx context.Context, call *Call) error

	// GetCall returns a call at a certain id and app name.
	GetCall(ctx context.Context, appName, callID string) (*Call, error)

	// GetCalls returns a list of calls that satisfy the given CallFilter. If no
	// calls exist, an empty list and a nil error are returned.
	GetCalls(ctx context.Context, filter *CallFilter) ([]*Call, error)
}
