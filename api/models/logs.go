package models

import (
	"context"
)

type FnLog interface {

	InsertLog(ctx context.Context, callID string, callLog string) error
	GetLog(ctx context.Context, callID string) (*FnCallLog, error)
	DeleteLog(ctx context.Context, callID string) error
}
