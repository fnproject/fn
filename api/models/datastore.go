package models

import (
	"context"
	"io"
)

type Datastore interface {
	// GetAppByID gets an App by ID.
	// Returns ErrAppsNotFound if no app is found.
	GetAppByID(ctx context.Context, appID string) (*App, error)

	// GetAppID gets an app ID by app name, ensures if app exists.
	// Returns ErrDatastoreEmptyAppName for empty appName.
	// Returns ErrAppsNotFound if no app is found.
	GetAppID(ctx context.Context, appName string) (string, error)

	// GetApps gets a slice of Apps, optionally filtered by name, and a cursor.
	// Missin filter or empty name will match all Apps.
	GetApps(ctx context.Context, filter *AppFilter) (*AppList, error)

	// InsertApp inserts an App. Returns ErrDatastoreEmptyApp when app is nil, and
	// ErrDatastoreEmptyAppName when app.Name is empty.
	// Returns ErrAppsAlreadyExists if an App by the same name already exists.
	InsertApp(ctx context.Context, app *App) (*App, error)

	// UpdateApp updates an App's Config. Returns ErrDatastoreEmptyApp when app is nil, and
	// ErrDatastoreEmptyAppName when app.Name is empty.
	// Returns ErrAppsNotFound if an App is not found.
	UpdateApp(ctx context.Context, app *App) (*App, error)

	// RemoveApp removes the App named appName. Returns ErrDatastoreEmptyAppName if appName is empty.
	// Returns ErrAppsNotFound if an App is not found.
	RemoveApp(ctx context.Context, appID string) error

	// InsertFn inserts a new function if one does not exist, applying any defaults necessary,
	InsertFn(ctx context.Context, fn *Fn) (*Fn, error)

	// UpdateFn  updates a function that exists under the same id.
	// ErrMissingName is func.Name is empty.
	UpdateFn(ctx context.Context, fn *Fn) (*Fn, error)

	// GetFns returns a list of funcs, and a cursor, applying any additional filters provided.
	GetFns(ctx context.Context, filter *FnFilter) (*FnList, error)

	// GetFnByID returns a function by ID. Returns ErrDatastoreEmptyFnID if fnID is empty.
	// Returns ErrFnsNotFound if a fn is not found.
	GetFnByID(ctx context.Context, fnID string) (*Fn, error)

	// RemoveFn removes a function. Returns ErrDatastoreEmptyFnID if fnID is empty.
	// Returns ErrFnsNotFound if a func is not found.
	RemoveFn(ctx context.Context, fnID string) error

	// InsertTrigger inserts a trigger. Returns ErrDatastoreEmptyTrigger when trigger is nil, and specific errors for each field
	// Returns ErrTriggerAlreadyExists if the exact apiID, fnID, source, type combination already exists
	InsertTrigger(ctx context.Context, trigger *Trigger) (*Trigger, error)

	//UpdateTrigger updates a trigger object in the data store
	UpdateTrigger(ctx context.Context, trigger *Trigger) (*Trigger, error)

	// Removes a Trigger. Returns field specific errors if they are empty.
	// Returns nil if successful
	RemoveTrigger(ctx context.Context, triggerID string) error

	// GetTriggerByID gets a trigger by it's id.
	// Returns ErrTriggerNotFound when no matching trigger is found
	GetTriggerByID(ctx context.Context, triggerID string) (*Trigger, error)

	// GetTriggers gets a list of triggers that match the specified filter
	// Return ErrDatastoreEmptyAppId if no AppID set in the filter
	GetTriggers(ctx context.Context, filter *TriggerFilter) (*TriggerList, error)

	// GetTriggerBySource loads a trigger by type and source ID - this is only needed when the data store is also used for agent read access
	GetTriggerBySource(ctx context.Context, appId string, triggerType, source string) (*Trigger, error)

	// implements io.Closer to shutdown
	io.Closer
}
