package models

import (
	"context"
	"io"
)

type LogStore interface {
	// InsertLog will insert the log at callID, overwriting if it previously
	// existed.
	InsertLog(ctx context.Context, appName, callID string, callLog io.Reader) error

	// GetLog will return the log at callID, an error will be returned if the log
	// cannot be found.
	// TODO it would be nice if this were an io.Reader...
	GetLog(ctx context.Context, appName, callID string) (*CallLog, error)

	// DeleteLog will remove the log at callID, it will not return an error if
	// the log does not exist before removal.
	DeleteLog(ctx context.Context, appName, callID string) error
}
