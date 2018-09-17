package models

import (
	"context"
	"io"
)

type LogStore interface {
	// InsertLog will insert the log at callID, overwriting if it previously
	// existed.
	InsertLog(ctx context.Context, call *Call, callLog io.Reader) error

	// GetLog will return the log at callID, an error will be returned if the log
	// cannot be found.
	GetLog(ctx context.Context, fnID, callID string) (io.Reader, error)

	// TODO we should probably allow deletion of a range of logs (also calls)?
	// common cases for deletion will be:
	// * fn gets nuked
	// * app gets nuked
	// * call+logs getting cleaned up periodically

	// InsertCall inserts a call into the datastore, it will error if the call already
	// exists.
	InsertCall(ctx context.Context, call *Call) error

	// GetCall2 returns a call at a certain id
	GetCall(ctx context.Context, fnID, callID string) (*Call, error)

	// GetCalls returns a list of calls that satisfy the given CallFilter. If no
	// calls exist, an empty list and a nil error are returned.
	GetCalls(ctx context.Context, filter *CallFilter) (*CallList, error)

	// Close will close any underlying connections as needed.
	// Close is not safe to be called from multiple threads.
	io.Closer
}
