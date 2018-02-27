package fnext

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
	BeforeAppDelete(ctx context.Context, appName string) error
	// AfterAppDelete called after deleting App in the database
	AfterAppDelete(ctx context.Context, appName string) error
	// BeforeAppGet called right before getting an app
	BeforeAppGet(ctx context.Context, appName string) error
	// AfterAppGet called after getting app from database
	AfterAppGet(ctx context.Context, app *models.App) error
	// BeforeAppsList called right before getting a list of all user's apps. Modify the filter to adjust what gets returned.
	BeforeAppsList(ctx context.Context, filter *models.AppFilter) error
	// AfterAppsList called after deleting getting a list of user's apps. apps is the result after applying AppFilter.
	AfterAppsList(ctx context.Context, apps []*models.App) error

	// TODO: WHAT IF THESE WERE CHANGE TO WRAPPERS INSTEAD OF LISTENERS, SORT OF LIKE MIDDLEWARE, EG
	// AppCreate(ctx, app, next func(ctx, app) or next.AppCreate(ctx, app)) <- where func is the InsertApp function (ie: the corresponding Datastore function)
	// Then instead of two two functions and modifying objects in the params, they get modified and then passed on. Eg:
	// AppCreate(ctx, app, next) (app *models.App, err error) {
	//     // do stuff before
	//     app.Name = app.Name + "-12345"
	//     app, err = next.AppCreate(ctx, app)
	//     // do stuff after if you want
	//     return app, err
	// }
}

// CallListener enables callbacks around Call events
type CallListener interface {
	// BeforeCall called before a function is executed
	BeforeCall(ctx context.Context, call *models.Call) error
	// AfterCall called after a function completes
	AfterCall(ctx context.Context, call *models.Call) error
}
