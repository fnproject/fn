package validator

import (
	"context"
	"io"

	"github.com/fnproject/fn/api/models"
)

func NewValidator(ls models.LogStore) models.LogStore {
	return &validator{ls}
}

type validator struct {
	models.LogStore
}

// callID or appID will never be empty.
func (v *validator) InsertLog(ctx context.Context, appID, callID string, callLog io.Reader) error {
	if callID == "" {
		return models.ErrDatastoreEmptyCallID
	}
	if appID == "" {
		return models.ErrMissingAppID
	}
	return v.LogStore.InsertLog(ctx, appID, callID, callLog)
}

// callID or appID will never be empty.
func (v *validator) GetLog(ctx context.Context, appID, callID string) (io.Reader, error) {
	if callID == "" {
		return nil, models.ErrDatastoreEmptyCallID
	}
	if appID == "" {
		return nil, models.ErrMissingAppID
	}
	return v.LogStore.GetLog(ctx, appID, callID)
}

// callID or appID will never be empty.
func (v *validator) InsertCall(ctx context.Context, call *models.Call) error {
	if call.ID == "" {
		return models.ErrDatastoreEmptyCallID
	}
	if call.AppID == "" {
		return models.ErrMissingAppID
	}
	return v.LogStore.InsertCall(ctx, call)
}

// callID or appID will never be empty.
func (v *validator) GetCall(ctx context.Context, appID, callID string) (*models.Call, error) {
	if callID == "" {
		return nil, models.ErrDatastoreEmptyCallID
	}
	if appID == "" {
		return nil, models.ErrMissingAppID
	}
	return v.LogStore.GetCall(ctx, appID, callID)
}
