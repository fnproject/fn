package extenders

import (
	"context"

	"github.com/fnproject/fn/api/models"
)

// AppListener is an interface used to inject custom code at key points in app lifecycle.
type AppListener interface {
	// BeforeAppCreate called right before creating App in the database
	BeforeAppCreate(ctx context.Context, app *models.App) error
	// AfterAppCreate called after creating App in the database
	AfterAppCreate(ctx context.Context, app *models.App) error
	// BeforeAppUpdate called right before updating App in the database
	BeforeAppUpdate(ctx context.Context, app *models.App) error
	// AfterAppUpdate called after updating App in the database
	AfterAppUpdate(ctx context.Context, app *models.App) error
	// BeforeAppDelete called right before deleting App in the database
	BeforeAppDelete(ctx context.Context, app *models.App) error
	// AfterAppDelete called after deleting App in the database
	AfterAppDelete(ctx context.Context, app *models.App) error
}

// CallListener enables callbacks around Call events
type CallListener interface {
	// BeforeCall called before a function is executed
	BeforeCall(ctx context.Context, call *models.Call) error
	// AfterCall called after a function completes
	AfterCall(ctx context.Context, call *models.Call) error
}
