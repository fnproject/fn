package models

import (
	"context"
)

type FnLog interface {
	// InsertLog will insert the log at callID, overwriting if it previously
	// existed.
	InsertLog(ctx context.Context, callID string, callLog string) error

	// GetLog will return the log at callID, an error will be returned if the log
	// cannot be found.
	GetLog(ctx context.Context, callID string) (*FnCallLog, error)

	// DeleteLog will remove the log at callID, it will not return an error if
	// the log does not exist before removal.
	DeleteLog(ctx context.Context, callID string) error
}
