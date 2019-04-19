package fnext

import (
	"context"

	"github.com/fnproject/fn/api/models"
)

// AppListener is an interface used to inject custom code at key points in the app lifecycle.
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
	// BeforeAppGet called right before getting an app
	BeforeAppGet(ctx context.Context, appID string) error
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

// FnListener enables callbacks around Fn events
type FnListener interface {
	// BeforeFnCreate called before fn created in the datastore
	BeforeFnCreate(ctx context.Context, fn *models.Fn) error
	// AfterFnCreate called after fn create in the datastore
	AfterFnCreate(ctx context.Context, fn *models.Fn) error
	// BeforeFnUpdate called before fn update in datastore
	BeforeFnUpdate(ctx context.Context, fn *models.Fn) error
	// AfterFnUpdate called after fn updated in datastore
	AfterFnUpdate(ctx context.Context, fn *models.Fn) error
	// BeforeFnDelete called before fn deleted from the datastore
	BeforeFnDelete(ctx context.Context, fnID string) error
	// AfterFnDelete called after fn deleted from the datastore
	AfterFnDelete(ctx context.Context, fnID string) error
}

// TriggerListener enables callbacks around Trigger events
type TriggerListener interface {
	// BeforeTriggerCreate called before trigger created in the datastore
	BeforeTriggerCreate(ctx context.Context, trigger *models.Trigger) error
	// AfterTriggerCreate called after trigger create in the datastore
	AfterTriggerCreate(ctx context.Context, trigger *models.Trigger) error
	// BeforeTriggerUpdate called before trigger update in datastore
	BeforeTriggerUpdate(ctx context.Context, trigger *models.Trigger) error
	// AfterTriggerUpdate called after trigger updated in datastore
	AfterTriggerUpdate(ctx context.Context, trigger *models.Trigger) error
	// BeforeTriggerDelete called before trigger deleted from the datastore
	BeforeTriggerDelete(ctx context.Context, triggerId string) error
	// AfterTriggerDelete called after trigger deleted from the datastore
	AfterTriggerDelete(ctx context.Context, triggerId string) error
}

// CallListener enables callbacks around Call events.
type CallListener interface {
	// BeforeCall called before a function is executed
	BeforeCall(ctx context.Context, call *models.Call) error
	// AfterCall called after a function completes
	AfterCall(ctx context.Context, call *models.Call) error
}
