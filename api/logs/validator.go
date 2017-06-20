package logs

import (
	"context"

	"gitlab-odx.oracle.com/odx/functions/api/models"
)

type FnLog interface {

	InsertLog(ctx context.Context, callID string, callLog string) error
	GetLog(ctx context.Context, callID string) (*models.FnCallLog, error)
	DeleteLog(ctx context.Context, callID string) error
}

type validator struct {
	fnl FnLog
}

func NewValidator(fnl FnLog) models.FnLog {
	return &validator{fnl}
}


func (v *validator) InsertLog(ctx context.Context, callID string, callLog string) error {
	return v.fnl.InsertLog(ctx, callID, callLog)
}

func (v *validator) GetLog(ctx context.Context, callID string) (*models.FnCallLog, error) {
	return v.fnl.GetLog(ctx, callID)
}

func (v *validator) DeleteLog(ctx context.Context, callID string) error {
	return v.fnl.DeleteLog(ctx, callID)
}
