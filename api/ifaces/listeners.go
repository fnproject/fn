package ifaces

import (
	"context"

	"github.com/iron-io/functions/api/models"
)

type AppCreateListener interface {
	// BeforeAppCreate called right before creating App in the database
	BeforeAppCreate(ctx context.Context, app *models.App) error
	// AfterAppCreate called after creating App in the database
	AfterAppCreate(ctx context.Context, app *models.App) error
}

type AppUpdateListener interface {
	// BeforeAppUpdate called right before updating App in the database
	BeforeAppUpdate(ctx context.Context, app *models.App) error
	// AfterAppUpdate called after updating App in the database
	AfterAppUpdate(ctx context.Context, app *models.App) error
}

type AppDeleteListener interface {
	// BeforeAppDelete called right before deleting App in the database
	BeforeAppDelete(ctx context.Context, appName string) error
	// AfterAppDelete called after deleting App in the database
	AfterAppDelete(ctx context.Context, appName string) error
}
