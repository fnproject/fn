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
func (v *validator) InsertLog(ctx context.Context, call *models.Call, callLog io.Reader) error {
	if call.ID == "" {
		return models.ErrDatastoreEmptyCallID
	}
	if call.AppID == "" && call.FnID == "" {
		return models.ErrMissingFnID
	}
	return v.LogStore.InsertLog(ctx, call, callLog)
}

// callID or appID will never be empty.
func (v *validator) GetLog(ctx context.Context, fnID, callID string) (io.Reader, error) {
	if callID == "" {
		return nil, models.ErrDatastoreEmptyCallID
	}
	if fnID == "" {
		return nil, models.ErrMissingFnID
	}
	return v.LogStore.GetLog(ctx, fnID, callID)
}

// callID or appID will never be empty.
func (v *validator) InsertCall(ctx context.Context, call *models.Call) error {
	if call.ID == "" {
		return models.ErrDatastoreEmptyCallID
	}
	if call.FnID == "" {
		return models.ErrMissingFnID
	}
	return v.LogStore.InsertCall(ctx, call)
}

// callID or appID will never be empty.
func (v *validator) GetCall(ctx context.Context, fnID, callID string) (*models.Call, error) {
	if callID == "" {
		return nil, models.ErrDatastoreEmptyCallID
	}
	if fnID == "" {
		return nil, models.ErrMissingFnID
	}
	return v.LogStore.GetCall(ctx, fnID, callID)
}
