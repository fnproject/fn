package models

import (
	"context"
)

type LogStore interface {
	// TODO TODO TODO BAD BUG BUG BUG WILL ROBINSON
	// TODO these need to take an app name or users can provide ids for
	// other users calls with their own app name and access their logs.

	// InsertLog will insert the log at callID, overwriting if it previously
	// existed.
	InsertLog(ctx context.Context, appName, callID string, callLog string) error

	// GetLog will return the log at callID, an error will be returned if the log
	// cannot be found.
	GetLog(ctx context.Context, appName, callID string) (*CallLog, error)

	// DeleteLog will remove the log at callID, it will not return an error if
	// the log does not exist before removal.
	DeleteLog(ctx context.Context, appName, callID string) error

	BatchDeleteLogs(ctx context.Context, appName string) error
}
