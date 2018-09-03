package validator

import (
	"context"
	"fmt"
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
func (v *validator) InsertLog(ctx context.Context, appID, fnID, callID string, callLog io.Reader) error {
	if callID == "" {
		return models.ErrDatastoreEmptyCallID
	}
	if appID == "" {
		return models.ErrMissingAppID
	}
	if fnID == "" {
		return models.ErrMissingFnID
	}
	fmt.Println("Inserting log")
	return v.LogStore.InsertLog(ctx, appID, fnID, callID, callLog)
}

// callID or appID will never be empty.
func (v *validator) GetLog(ctx context.Context, callID string) (io.Reader, error) {
	if callID == "" {
		return nil, models.ErrDatastoreEmptyCallID
	}
	return v.LogStore.GetLog(ctx, callID)
}

// callID or appID will never be empty.
func (v *validator) InsertCall(ctx context.Context, call *models.Call) error {
	if call.ID == "" {
		return models.ErrDatastoreEmptyCallID
	}
	if call.AppID == "" {
		return models.ErrMissingAppID
	}
	if call.FnID == "" {
		return models.ErrMissingFnID
	}
	return v.LogStore.InsertCall(ctx, call)
}

// callID or appID will never be empty.
func (v *validator) GetCall(ctx context.Context, callID string) (*models.Call, error) {
	if callID == "" {
		return nil, models.ErrDatastoreEmptyCallID
	}
	return v.LogStore.GetCall(ctx, callID)
}
