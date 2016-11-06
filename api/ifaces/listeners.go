package ifaces

import (
	"context"

	"github.com/iron-io/functions/api/models"
)

type AppListener interface {
	// BeforeAppUpdate called right before storing App in the database
	BeforeAppUpdate(ctx context.Context, app *models.App) error
	// AfterAppUpdate called after storing App in the database
	AfterAppUpdate(ctx context.Context, app *models.App) error
}
