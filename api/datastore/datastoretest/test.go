package datastoretest

// Data store correctness tests -
// These tests run validation tests on an underlying data store implementation and can be re-used for new data stores.
// TODO: Generalize some tests around metadata (updated_created,ids)
// TODO: Generalize tests around pagination and filtering
import (
	"bytes"
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func setLogBuffer() *bytes.Buffer {
	var buf bytes.Buffer
	buf.WriteByte('\n')
	logrus.SetOutput(&buf)
	gin.DefaultErrorWriter = &buf
	gin.DefaultWriter = &buf
	log.SetOutput(&buf)
	return &buf
}

//ResourceProvider provides an abstraction for supplying data store tests with
// appropriate initial testing objects for running tests
// Use the resource calls to supply objects with (e.g.) middleware enforced annotations set on them
// Use DefaultCtx to override custom middleware-supplied context variables
type ResourceProvider interface {
	// ValidApp returns a valid app to use for inserts
	ValidApp() *models.App
	// ValidFn returns a valid fn to use for inserts
	ValidFn(appId string) *models.Fn
	// ValidFn returns a valid fn to use for inserts
	ValidRoute(appId string) *models.Route
	// ValidTrigger returns a valid trigger  to use for inserts
	ValidTrigger(appId string, fnId string) *models.Trigger

	// DefaultCtx returns a context object (which may have custom attributes set)
	// this may be used (e.g.) to pass on tenancy and user details that would originate from a middleware to your data store
	DefaultCtx() context.Context
}

// BasicResourceProvider supplies simple objects and can be used as a base for custom resource providers
type BasicResourceProvider struct {
	idCount int32
}

// DataStoreFunc provides an instance of a data store
type DataStoreFunc func(*testing.T) models.Datastore

func NewBasicResourceProvider() ResourceProvider {
	return &BasicResourceProvider{}
}

func (brp *BasicResourceProvider) NextID() int32 {
	return atomic.AddInt32(&brp.idCount, 1)
}

func (brp *BasicResourceProvider) DefaultCtx() context.Context {
	return context.Background()
}

// Creates a valid app which always has a sequential named
func (brp *BasicResourceProvider) ValidApp() *models.App {

	app := &models.App{
		Name: fmt.Sprintf("app_%09d", brp.NextID()),
	}
	return app
}

func (brp *BasicResourceProvider) ValidTrigger(appId, funcId string) *models.Trigger {

	trigger := &models.Trigger{
		Name:   fmt.Sprintf("trigger_%09d", brp.NextID()),
		AppID:  appId,
		FnID:   funcId,
		Type:   "http",
		Source: "ASource",
	}

	return trigger
}

// Creates a valid route which always has a sequential named
func (brp *BasicResourceProvider) ValidRoute(appId string) *models.Route {
	testRoute := &models.Route{
		AppID:       appId,
		Path:        fmt.Sprintf("/test_%09d", brp.NextID()),
		Image:       "fnproject/fn-test-utils",
		Type:        "sync",
		Format:      "http",
		Timeout:     models.DefaultTimeout,
		IdleTimeout: models.DefaultIdleTimeout,
		Memory:      models.DefaultMemory,
	}
	return testRoute
}

func (brp *BasicResourceProvider) ValidFn(appId string) *models.Fn {
	return &models.Fn{
		AppID:  appId,
		Name:   fmt.Sprintf("test_%09d", brp.NextID()),
		Image:  "fnproject/fn-test-utils",
		Format: "http",
		ResourceConfig: models.ResourceConfig{
			Timeout:     models.DefaultTimeout,
			IdleTimeout: models.DefaultIdleTimeout,
			Memory:      models.DefaultMemory,
		},
	}
}

type Harness struct {
	ctx    context.Context
	t      *testing.T
	ds     models.Datastore
	appIds []string
}

func (h *Harness) GivenAppInDb(app *models.App) *models.App {
	a, err := h.ds.InsertApp(h.ctx, app)
	if err != nil {
		h.t.Fatal("failed to create app", err)
		return nil
	}
	h.AppForDeletion(a)
	return a
}

func (h *Harness) GivenRouteInDb(rt *models.Route) *models.Route {
	r, err := h.ds.InsertRoute(h.ctx, rt)
	if err != nil {
		h.t.Fatal("failed to create rt", err)
		return nil
	}
	return r
}

func (h *Harness) Cleanup() {
	for _, appId := range h.appIds {
		err := h.ds.RemoveApp(h.ctx, appId)
		if err != nil && err != models.ErrAppsNotFound {
			h.t.Fatalf("Failed to cleanup app %s %s", appId, err)
		}
	}
}

func (h *Harness) GivenFnInDb(validFunc *models.Fn) *models.Fn {
	fn, err := h.ds.InsertFn(h.ctx, validFunc)
	if err != nil {
		h.t.Fatalf("Failed to insert function %s", err)
		return nil
	}
	return fn

}

func (h *Harness) GivenTriggerInDb(validTrigger *models.Trigger) *models.Trigger {
	trigger, err := h.ds.InsertTrigger(h.ctx, validTrigger)
	if err != nil {
		h.t.Fatalf("Failed to insert trigger %s", err)
		return nil
	}
	return trigger

}
func (h *Harness) AppForDeletion(app *models.App) {
	h.appIds = append(h.appIds, app.ID)
}

func NewHarness(t *testing.T, ctx context.Context, ds models.Datastore) *Harness {
	return &Harness{
		ctx: ctx,
		t:   t,
		ds:  ds,
	}
}

func RunAppsTest(t *testing.T, dsf DataStoreFunc, rp ResourceProvider) {

	ds := dsf(t)
	ctx := rp.DefaultCtx()

	t.Run("apps", func(t *testing.T) {
		// Testing insert app

		t.Run("insert missing app name fails", func(t *testing.T) {
			_, err := ds.InsertApp(ctx, &models.App{})
			if err != models.ErrMissingName {
				t.Fatalf("expected error `%v`, but it was `%v`", models.ErrMissingName, err)
			}
		})

		t.Run("insert sets created time and updated time ", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()

			start := time.Now()
			returnedApp, err := ds.InsertApp(ctx, rp.ValidApp())
			if err != nil {
				t.Fatalf("Expected succcess, got %s", err)
			}
			h.AppForDeletion(returnedApp)

			if !time.Time(returnedApp.CreatedAt).After(start) {
				t.Fatalf("expected created to be set %s", returnedApp.CreatedAt)
			}

			if !time.Time(returnedApp.UpdatedAt).After(start) {
				t.Fatalf("expected updated to be set  %s", returnedApp.UpdatedAt)
			}
		})

		t.Run("update sets update time ", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()

			// Set a config var
			testApp := h.GivenAppInDb(rp.ValidApp())

			time.Sleep(10 * time.Millisecond)
			testApp.Config = map[string]string{"TEST": "1"}
			updated, err := ds.UpdateApp(ctx, testApp)

			if err != nil {
				t.Fatalf("didn't update app %s", err)
			}

			if !time.Time(updated.UpdatedAt).After(time.Time(testApp.UpdatedAt)) {
				t.Errorf("Expected updated time to be after original %s !> %s", updated.UpdatedAt, testApp.UpdatedAt)
			}

		})

		t.Run("update no-op", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()

			// Set a config var
			testApp := h.GivenAppInDb(rp.ValidApp())
			time.Sleep(1 * time.Millisecond)
			updated, err := ds.UpdateApp(ctx, testApp)
			if err != nil {
				t.Fatalf("Expected succes got %s", err)
			}
			if updated == testApp {
				t.Fatalf("Update should return a new value")
			}
			if updated.UpdatedAt.String() != testApp.UpdatedAt.String() {
				t.Fatalf("Expected app not to be updated but update times weren't equal %s != %s", updated.UpdatedAt, testApp.UpdatedAt)
			}

		})

		t.Run("update with new config var", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()

			// Set a config var
			testApp := h.GivenAppInDb(rp.ValidApp())
			testApp.Config = map[string]string{"TEST": "1"}
			updated, err := ds.UpdateApp(ctx, testApp)
			if err != nil {
				t.Fatalf("error when updating app: %v", err)
			}
			expected := &models.App{ID: testApp.ID, Name: testApp.Name, Config: map[string]string{"TEST": "1"}}
			if !expected.EqualsWithAnnotationSubset(updated) {
				t.Fatalf("expected updated `%v` but got `%v`", expected, updated)
			}
		})

		t.Run("set multiple config vars", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()

			testApp := h.GivenAppInDb(rp.ValidApp())
			testApp.Config = map[string]string{"TEST": "1"}
			updated, err := ds.UpdateApp(ctx, testApp)
			if err != nil {
				t.Fatalf("error when updating app: %v", err)
			}
			// Set a different var (without clearing the existing)
			another := testApp.Clone()
			another.Config = map[string]string{"OTHER": "TEST"}
			updated, err = ds.UpdateApp(ctx, another)
			if err != nil {
				t.Fatalf("error when updating app: %v", err)
			}
			expected := &models.App{Name: testApp.Name, ID: testApp.ID, Config: map[string]string{"TEST": "1", "OTHER": "TEST"}}
			if !expected.EqualsWithAnnotationSubset(updated) {
				t.Fatalf("expected updated `%v` but got `%v`", expected, updated)
			}
		})

		t.Run("Insert duplicate named app", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()

			testApp := h.GivenAppInDb(rp.ValidApp())

			testApp2 := rp.ValidApp()
			testApp2.Name = testApp.Name

			_, err := ds.InsertApp(ctx, testApp2)
			if err != models.ErrAppsAlreadyExists {
				t.Fatalf("Expecting duplicate error got %s", err)
			}
		})

		t.Run("Update name is immutable", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()

			testApp := h.GivenAppInDb(rp.ValidApp())
			testApp.Name = "other"

			_, err := ds.UpdateApp(ctx, testApp)
			if err != models.ErrAppsNameImmutable {
				t.Fatalf("Expecting name immutable %s", err)
			}
		})

		t.Run("remove config var", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()

			app := rp.ValidApp()
			app.Config = map[string]string{"OTHER": "TEST", "TEST": "1"}

			// Delete a var
			testApp := h.GivenAppInDb(app)
			testApp.Config = map[string]string{"TEST": ""}
			updated, err := ds.UpdateApp(ctx, testApp)
			if err != nil {
				t.Fatalf("error when updating app: %v", err)
			}
			expected := &models.App{Name: testApp.Name, ID: testApp.ID, Config: map[string]string{"OTHER": "TEST"}}
			if !expected.EqualsWithAnnotationSubset(updated) {
				t.Fatalf("expected updated `%#v` but got `%#v`", expected, updated)
			}
		})

		// Testing get app

		t.Run("Get with empty App ID", func(t *testing.T) {
			_, err := ds.GetAppByID(ctx, "")
			if err != models.ErrAppsMissingID {
				t.Fatalf("expected error to be %v, but it was %s", models.ErrAppsMissingID, err)
			}
		})

		t.Run("Get app by ID ", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()

			testApp := h.GivenAppInDb(rp.ValidApp())
			app, err := ds.GetAppByID(ctx, testApp.ID)
			if err != nil {
				t.Fatalf("error: %s", err)
			}
			if app.Name != testApp.Name {
				t.Fatalf("expected `app.Name` to be `%s` but it was `%s`", app.Name, testApp.Name)
			}
		})

		t.Run("List apps", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())

			// Testing list apps
			apps, err := ds.GetApps(ctx, &models.AppFilter{PerPage: 100})
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if len(apps) == 0 {
				t.Fatal("expected result count to be greater than 0")
			}
			for _, app := range apps {
				if app.Name == testApp.Name {
					return
				}
			}
			t.Fatalf("expected app list to contain app %s, got %#v", testApp.Name, apps)
		})

		t.Run("Simple Pagination", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			// test pagination stuff (ordering / limits / cursoring)
			a1 := h.GivenAppInDb(rp.ValidApp())
			a2 := h.GivenAppInDb(rp.ValidApp())
			a3 := h.GivenAppInDb(rp.ValidApp())

			apps, err := ds.GetApps(ctx, &models.AppFilter{PerPage: 1})
			if err != nil {
				t.Fatalf(" error: %s", err)
			}
			if len(apps) != 1 {
				t.Fatalf(" expected result count to be 1 but got %d", len(apps))
			} else if apps[0].Name != a1.Name {
				t.Fatalf(" expected `app.Name` to be `%s` but it was `%s`", a1.Name, apps[0].Name)
			}

			apps, err = ds.GetApps(ctx, &models.AppFilter{PerPage: 100, Cursor: apps[0].Name})
			if err != nil {
				t.Fatalf(" error: %s", err)
			}
			if len(apps) != 2 {
				t.Fatalf(" expected result count to be 2 but got %d", len(apps))
			} else if apps[0].Name != a2.Name {
				t.Fatalf(" expected `app.Name` to be `%s` but it was `%s`", a2.Name, apps[0].Name)
			} else if apps[1].Name != a3.Name {
				t.Fatalf(" expected `app.Name` to be `%s` but it was `%s`", a3.Name, apps[1].Name)
			}

			a4 := h.GivenAppInDb(rp.ValidApp())

			apps, err = ds.GetApps(ctx, &models.AppFilter{PerPage: 100})
			if err != nil {
				t.Fatalf(" error: %s", err)
			}
			if len(apps) != 4 {
				t.Fatalf(" expected result count to be 4 but got %d", len(apps))
			} else if apps[3].Name != a4.Name {
				t.Fatalf(" expected `app.Name` to be `%s` but it was `%s`", a4.Name, apps[0].Name)
			}

		})

		t.Run("delete app with empty Id", func(t *testing.T) {
			// Testing app delete
			err := ds.RemoveApp(ctx, "")
			if err != models.ErrAppsMissingID {
				t.Fatalf("expected error `%v`, but it was `%v`", models.ErrAppsMissingID, err)
			}
		})

		t.Run("delete app results in app not existing", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()

			testApp := h.GivenAppInDb(rp.ValidApp())
			err := ds.RemoveApp(ctx, testApp.ID)
			if err != nil {
				t.Fatalf("error: %s", err)
			}
			app, err := ds.GetAppByID(ctx, testApp.ID)
			if err != models.ErrAppsNotFound {
				t.Fatalf("expected error `%v`, but it was `%v`", models.ErrAppsNotFound, err)
			}
			if app != nil {
				t.Log(err.Error())
				t.Fatal("failed to remove the app, app should be gone already")
			}
		})

		t.Run("cannot update non-existant app ", func(t *testing.T) {
			missingApp := &models.App{
				ID:   "nonexistant",
				Name: "nonexistant",
			}
			_, err := ds.UpdateApp(ctx, missingApp)
			if err != models.ErrAppsNotFound {
				t.Fatalf("expected error `%v`, but it was `%v`", models.ErrAppsNotFound, err)
			}
		})

	})

}

func RunRoutesTest(t *testing.T, dsf DataStoreFunc, rp ResourceProvider) {

	ds := dsf(t)
	ctx := rp.DefaultCtx()

	t.Run("routes", func(t *testing.T) {

		t.Run("empty val", func(t *testing.T) {
			_, err := ds.InsertRoute(ctx, nil)
			if err != models.ErrDatastoreEmptyRoute {
				t.Fatalf("expected error `%v`, but it was `%v`", models.ErrDatastoreEmptyRoute, err)
			}

		})

		t.Run("Insert with non-existant app ", func(t *testing.T) {

			newTestRoute := rp.ValidRoute("notreal")
			_, err := ds.InsertRoute(ctx, newTestRoute)
			if err != models.ErrAppsNotFound {
				t.Fatalf("expected error `%v`, but it was `%v`", models.ErrAppsNotFound, err)
			}
		})

		t.Run("Insert duplicate route fails", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())
			testRoute := rp.ValidRoute(testApp.ID)
			h.GivenRouteInDb(testRoute)

			_, err := ds.InsertRoute(ctx, testRoute)
			if err != models.ErrRoutesAlreadyExists {
				t.Fatalf("expected error to be `%v`, but it was `%v`", models.ErrRoutesAlreadyExists, err)
			}
		})

		// Testing get
		t.Run("get route with empty path", func(t *testing.T) {
			_, err := ds.GetRoute(ctx, id.New().String(), "")
			if err != models.ErrRoutesMissingPath {
				t.Fatalf("expected error `%v`, but it was `%v`", models.ErrRoutesMissingPath, err)
			}
		})

		t.Run("get route with empty app id", func(t *testing.T) {

			_, err := ds.GetRoute(ctx, "", "a")
			if err != models.ErrRoutesMissingAppID {
				t.Fatalf("expected error `%v`, but it was `%v`", models.ErrRoutesMissingAppID, err)
			}
		})
		t.Run("get valid route", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())
			testRoute := h.GivenRouteInDb(rp.ValidRoute(testApp.ID))

			route, err := ds.GetRoute(ctx, testApp.ID, testRoute.Path)
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if !route.Equals(testRoute) {
				t.Fatalf("expected to insert:\n%v\nbut got:\n%v", testRoute, *route)
			}
		})

		// Testing update
		t.Run("update route set headers and config", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())
			testRoute := h.GivenRouteInDb(rp.ValidRoute(testApp.ID))

			// Update some fields, and add 3 configs and 3 headers.
			updated, err := ds.UpdateRoute(ctx, &models.Route{
				AppID:   testApp.ID,
				Path:    testRoute.Path,
				Timeout: 2,
				Config: map[string]string{
					"FIRST":  "1",
					"SECOND": "2",
					"THIRD":  "3",
				},
				Headers: models.Headers{
					"First":  []string{"test"},
					"Second": []string{"test", "test"},
					"Third":  []string{"test", "test2"},
				},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			expected := &models.Route{
				// unchanged
				AppID:       testApp.ID,
				Path:        testRoute.Path,
				Image:       "fnproject/fn-test-utils",
				Type:        "sync",
				Format:      "http",
				IdleTimeout: testRoute.IdleTimeout,
				Memory:      testRoute.Memory,
				CPUs:        testRoute.CPUs,
				// updated
				Timeout: 2,
				Config: map[string]string{
					"FIRST":  "1",
					"SECOND": "2",
					"THIRD":  "3",
				},
				Headers: models.Headers{
					"First":  []string{"test"},
					"Second": []string{"test", "test"},
					"Third":  []string{"test", "test2"},
				},
			}
			if !updated.Equals(expected) {
				t.Fatalf("expected updated `%v` but got `%v`", expected, updated)
			}
		})

		t.Run("update route modify headers and config", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())
			testRoute := rp.ValidRoute(testApp.ID)
			testRoute.Config = map[string]string{
				"FIRST":  "1",
				"SECOND": "2",
				"THIRD":  "3",
			}
			testRoute.Headers = models.Headers{
				"First":  []string{"test"},
				"Second": []string{"test", "test"},
				"Third":  []string{"test", "test2"},
			}
			testRoute.Timeout = 2
			h.GivenRouteInDb(testRoute)

			// Update a config var, remove another. Add one Header, remove another.
			updated, err := ds.UpdateRoute(ctx, &models.Route{
				AppID: testRoute.AppID,
				Path:  testRoute.Path,
				Config: map[string]string{
					"FIRST":  "first",
					"SECOND": "",
					"THIRD":  "3",
				},
				Headers: models.Headers{
					"First":  []string{"test2"},
					"Second": nil,
				},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			expected := &models.Route{
				// unchanged
				AppID:       testRoute.AppID,
				Path:        testRoute.Path,
				Image:       "fnproject/fn-test-utils",
				Type:        "sync",
				Format:      "http",
				Timeout:     2,
				Memory:      testRoute.Memory,
				CPUs:        testRoute.CPUs,
				IdleTimeout: testRoute.IdleTimeout,
				// updated
				Config: map[string]string{
					"FIRST": "first",
					"THIRD": "3",
				},
				Headers: models.Headers{
					"First": []string{"test2"},
					"Third": []string{"test", "test2"},
				},
			}
			if !updated.Equals(expected) {
				t.Fatalf("expected updated:\n`%#v`\nbut got:\n`%#v`", expected, updated)
			}
		})

		t.Run("simple pagination", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())
			testRoute := h.GivenRouteInDb(rp.ValidRoute(testApp.ID))

			// Testing list fns
			routes, err := ds.GetRoutesByApp(ctx, testApp.ID, &models.RouteFilter{PerPage: 1})
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if len(routes) == 0 {
				t.Fatal("expected result count to be greater than 0")
			}
			if routes[0] == nil {
				t.Fatalf("expected non-nil route")
			} else if routes[0].Path != testRoute.Path {
				t.Fatalf("expected `app.Name` to be `%s` but it was `%s`", testRoute.Path, routes[0].Path)
			}

			routes, err = ds.GetRoutesByApp(ctx, testApp.ID, &models.RouteFilter{Image: testRoute.Image, PerPage: 1})
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if len(routes) == 0 {
				t.Fatal("expected result count to be greater than 0")
			}
			if routes[0] == nil {
				t.Fatalf("expected non-nil route")
			} else if routes[0].Path != testRoute.Path {
				t.Fatalf("expected `route.Path` to be `%s` but it was `%s`", testRoute.Path, routes[0].Path)
			}

		})

		t.Run("pagination on empty app is invalid", func(t *testing.T) {
			_, err := ds.GetRoutesByApp(ctx, "", &models.RouteFilter{PerPage: 1})
			if err != models.ErrRoutesMissingAppID {
				t.Fatalf("Expecting app ID error, got %s", err)
			}
		})

		t.Run("pagination on non-existant app returns no routes", func(t *testing.T) {
			routes, err := ds.GetRoutesByApp(ctx, id.New().String(), &models.RouteFilter{PerPage: 1})
			if err != nil {
				t.Fatalf("error: %s", err)
			}
			if len(routes) != 0 {
				t.Fatalf("expected result count to be 0 but got %d", len(routes))
			}
		})

		t.Run("pagination on routes return rotues in order ", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())

			r1 := h.GivenRouteInDb(rp.ValidRoute(testApp.ID))
			r2 := h.GivenRouteInDb(rp.ValidRoute(testApp.ID))
			r3 := h.GivenRouteInDb(rp.ValidRoute(testApp.ID))

			routes, err := ds.GetRoutesByApp(ctx, testApp.ID, &models.RouteFilter{PerPage: 1})
			if err != nil {
				t.Fatalf("error: %s", err)
			}
			if len(routes) != 1 {
				t.Fatalf("expected result count to be 1 but got %d", len(routes))
			} else if routes[0].Path != r1.Path {
				t.Fatalf("expected `route.Path` to be `%s` but it was `%s`", r1.Path, routes[0].Path)
			}

			routes, err = ds.GetRoutesByApp(ctx, testApp.ID, &models.RouteFilter{PerPage: 2, Cursor: routes[0].Path})
			if err != nil {
				t.Fatalf("error: %s", err)
			}

			if len(routes) != 2 {
				t.Fatalf("expected result count to be 2 but got %d", len(routes))
			} else if routes[0].Path != r2.Path {
				t.Fatalf("expected `route.Path` to be `%s` but it was `%s`", r2.Path, routes[0].Path)
			} else if routes[1].Path != r3.Path {
				t.Fatalf("expected `route.Path` to be `%s` but it was `%s`", r3.Path, routes[1].Path)
			}

			r4 := rp.ValidRoute(testApp.ID)
			r4.Path = "/abcdefg" // < /test lexicographically, but not in length

			h.GivenRouteInDb(r4)

			routes, err = ds.GetRoutesByApp(ctx, testApp.ID, &models.RouteFilter{PerPage: 100})
			if err != nil {
				t.Fatalf("error: %s", err)
			}
			if len(routes) != 4 {
				t.Fatalf("expected result count to be 4 but got %d", len(routes))
			} else if routes[0].Path != r4.Path {
				t.Fatalf("expected `route.Path` to be `%s` but it was `%s`", r4.Path, routes[0].Path)
			}
		})

		t.Run("remove route with empty app ID", func(t *testing.T) {

			// Testing route delete
			err := ds.RemoveRoute(ctx, "", "")
			if err != models.ErrRoutesMissingAppID {
				t.Fatalf("expected error `%v`, but it was `%v`", models.ErrRoutesMissingAppID, err)
			}

		})

		t.Run("remove route with empty app path", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())

			err := ds.RemoveRoute(ctx, testApp.ID, "")
			if err != models.ErrRoutesMissingPath {
				t.Fatalf("expected error `%v`, but it was `%v`", models.ErrRoutesMissingPath, err)
			}
		})

		t.Run("remove valid route removes route ", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())
			testRoute := h.GivenRouteInDb(rp.ValidRoute(testApp.ID))

			err := ds.RemoveRoute(ctx, testApp.ID, testRoute.Path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			route, err := ds.GetRoute(ctx, testApp.ID, testRoute.Path)
			if err != nil && err != models.ErrRoutesNotFound {
				t.Fatalf("expected error `%v`, but it was `%v`", models.ErrRoutesNotFound, err)
			}
			if route != nil {
				t.Fatalf("failed to remove the route: %v", route)
			}

			_, err = ds.UpdateRoute(ctx, &models.Route{
				AppID: testApp.ID,
				Path:  testRoute.Path,
				Image: "test",
			})
			if err != models.ErrRoutesNotFound {
				t.Fatalf("expected error to be `%v`, but it was `%v`", models.ErrRoutesNotFound, err)
			}
		})
	})
}

func RunFnsTest(t *testing.T, dsf DataStoreFunc, rp ResourceProvider) {

	ds := dsf(t)
	ctx := rp.DefaultCtx()

	t.Run("Fns", func(t *testing.T) {

		// Testing insert fn
		t.Run("empty function", func(t *testing.T) {
			_, err := ds.InsertFn(ctx, nil)
			if err != models.ErrDatastoreEmptyFn {
				t.Fatalf("expected error `%v`, but it was `%v`", models.ErrDatastoreEmptyFn, err)
			}

		})

		t.Run("invalid app ID", func(t *testing.T) {
			testFn := rp.ValidFn("notreal")
			_, err := ds.InsertFn(ctx, testFn)
			if err != models.ErrAppsNotFound {
				t.Fatalf("expected error `%v`, but it was `%v`", models.ErrAppsNotFound, err)
			}

		})

		t.Run("non-empty function ID", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())

			testFn := rp.ValidFn(testApp.ID)
			testFn.ID = "abc"

			_, err := ds.InsertFn(ctx, testFn)
			if err != models.ErrFnsIDProvided {
				t.Fatalf("expected error `%v`, but it was `%v`", models.ErrFnsIDProvided, err)
			}
		})

		t.Run("insert valid func", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())

			testFn := rp.ValidFn(testApp.ID)
			testFn, err := ds.InsertFn(ctx, testFn)
			if err != nil {
				t.Fatalf("error when storing perfectly good fn: %s", err)
			}
		})

		// Testing get
		t.Run("Get with empty function ID", func(t *testing.T) {
			_, err := ds.GetFnByID(ctx, "")
			if err != models.ErrDatastoreEmptyFnID {
				t.Fatalf("expected error `%v`, but it was `%v`", models.ErrDatastoreEmptyFnID, err)
			}
		})

		t.Run("Get with valid function", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())
			testFn := h.GivenFnInDb(rp.ValidFn(testApp.ID))
			fn, err := ds.GetFnByID(ctx, testFn.ID)
			if err != nil {
				t.Fatalf("unexpected error %v : %s", err, testFn.ID)
			}
			if !testFn.EqualsWithAnnotationSubset(fn) {
				t.Fatalf("expected to get the right func:\n%v\nbut got:\n%v", testFn, fn)
			}
		})

		// Testing update
		t.Run("Update function add values", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())
			testFn := h.GivenFnInDb(rp.ValidFn(testApp.ID))

			// Update some fields, and add 3 configs
			updated, err := ds.UpdateFn(ctx, &models.Fn{
				ID:    testFn.ID,
				Name:  testFn.Name,
				AppID: testFn.AppID,
				Config: map[string]string{
					"FIRST":  "1",
					"SECOND": "2",
					"THIRD":  "3",
				},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			expected := &models.Fn{
				// unchanged
				ID:     testFn.ID,
				Name:   testFn.Name,
				AppID:  testApp.ID,
				Image:  "fnproject/fn-test-utils",
				Format: "http",
				ResourceConfig: models.ResourceConfig{
					Timeout:     testFn.Timeout,
					IdleTimeout: testFn.IdleTimeout,
					Memory:      testFn.Memory,
				},
				// updated
				Config: map[string]string{
					"FIRST":  "1",
					"SECOND": "2",
					"THIRD":  "3",
				},
			}
			if !expected.EqualsWithAnnotationSubset(updated) {
				t.Fatalf("expected updated `%#v` but got `%#v`", expected, updated)
			}

		})

		t.Run("Update function modify and remove values", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())
			fn := rp.ValidFn(testApp.ID)

			fn.Config = map[string]string{
				"FIRST":  "1",
				"SECOND": "2",
				"THIRD":  "3",
			}

			testFn := h.GivenFnInDb(fn)

			// Update a config var, remove another. Add one Header, remove another.
			updated, err := ds.UpdateFn(ctx, &models.Fn{
				ID:    testFn.ID,
				Name:  testFn.Name,
				AppID: testFn.AppID,
				Config: map[string]string{
					"FIRST":  "first",
					"SECOND": "",
					"THIRD":  "3",
				},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			expected := &models.Fn{
				// unchanged
				ID:     testFn.ID,
				Name:   testFn.Name,
				AppID:  testApp.ID,
				Image:  "fnproject/fn-test-utils",
				Format: "http",
				ResourceConfig: models.ResourceConfig{
					Timeout:     testFn.Timeout,
					IdleTimeout: testFn.IdleTimeout,
					Memory:      testFn.Memory,
				},
				// updated
				Config: map[string]string{
					"FIRST": "first",
					"THIRD": "3",
				},
			}
			if !expected.EqualsWithAnnotationSubset(updated) {
				t.Fatalf("expected updated:\n`%v`\nbut got:\n`%v`", expected, updated)
			}
		})

		t.Run("basic pagination no functions", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())
			// Testing list fns
			fns, err := ds.GetFns(ctx, &models.FnFilter{AppID: testApp.ID, PerPage: 1})
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if len(fns) != 0 {
				t.Fatal("expected result count to be  0")
			}
		})

		t.Run("basic pagination with funcs", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())
			f1 := h.GivenFnInDb(rp.ValidFn(testApp.ID))
			f2 := h.GivenFnInDb(rp.ValidFn(testApp.ID))
			f3 := h.GivenFnInDb(rp.ValidFn(testApp.ID))

			// Testing list fns
			fns, err := ds.GetFns(ctx, &models.FnFilter{AppID: testApp.ID})
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if len(fns) != 3 {
				t.Fatalf("expected result count to be 3, but was %d", len(fns))
			}
			fns, err = ds.GetFns(ctx, &models.FnFilter{AppID: testApp.ID, PerPage: 1})
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if len(fns) != 1 {
				t.Fatalf("expected result count to be 1, but was %d", len(fns))
			}

			if !f1.EqualsWithAnnotationSubset(fns[0]) {
				t.Fatalf("Expecting function to be %#v but was %#v", f1, fns[0])
			}

			fns, err = ds.GetFns(ctx, &models.FnFilter{AppID: testApp.ID, PerPage: 2, Cursor: fns[0].Name})
			if err != nil {
				t.Fatalf("error: %s", err)
			}
			if len(fns) != 2 {
				t.Fatalf("expected result count to be 2 but got %d", len(fns))
			} else if !f2.EqualsWithAnnotationSubset(fns[0]) {
				t.Fatalf("expected `func.Name` to be `%#v` but it was `%#v`", f2, fns[0])
			} else if !f3.EqualsWithAnnotationSubset(fns[1]) {
				t.Fatalf("expected `func.Name` to be `%#v` but it was `%#v`", f3, fns[1])
			}
		})

		t.Run("delete with empty fn name", func(t *testing.T) {
			// Testing func delete
			err := ds.RemoveFn(ctx, "")
			if err != models.ErrDatastoreEmptyFnID {
				t.Fatalf("expected error `%v`, but it was `%v`", models.ErrDatastoreEmptyFnID, err)
			}

		})

		t.Run("delete valid fn", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())
			testFn := h.GivenFnInDb(rp.ValidFn(testApp.ID))
			err := ds.RemoveFn(ctx, testFn.ID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			fn, err := ds.GetFnByID(ctx, testFn.ID)
			if err != nil && err != models.ErrFnsNotFound {
				t.Fatalf("expected error `%v`, but it was `%v`", models.ErrFnsNotFound, err)
			}
			if fn != nil {
				t.Fatalf("failed to remove the func: %v", fn)
			}
		})

	})
}

func RunTriggersTest(t *testing.T, dsf DataStoreFunc, rp ResourceProvider) {
	t.Run("triggers", func(t *testing.T) {
		ds := dsf(t)
		ctx := rp.DefaultCtx()

		// Testing insert trigger
		t.Run("insert invalid app ID", func(t *testing.T) {
			newTestTrigger := rp.ValidTrigger("notreal", "fnId")
			_, err := ds.InsertTrigger(ctx, newTestTrigger)
			if err != models.ErrAppsNotFound {
				t.Fatalf("expected error `%v`, but it was `%v`", models.ErrAppsNotFound, err)
			}
		})

		t.Run("invalid func ID", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())
			_, err := ds.InsertTrigger(ctx, rp.ValidTrigger(testApp.ID, "notReal"))
			if err != models.ErrFnsNotFound {
				t.Fatalf("expected error `%v`, but it was `%v`", models.ErrFnsNotFound, err)
			}
		})

		t.Run("duplicate name", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())
			testFn := h.GivenFnInDb(rp.ValidFn(testApp.ID))
			newTrigger := rp.ValidTrigger(testApp.ID, testFn.ID)
			insertedTrigger, err := ds.InsertTrigger(ctx, newTrigger)
			if err != nil {
				t.Fatalf("error when storing new trigger: %s", err)
			}
			newTrigger.ID = insertedTrigger.ID
			if !newTrigger.EqualsWithAnnotationSubset(insertedTrigger) {
				t.Errorf("Expecting returned trigger %#v to equal %#v", insertedTrigger, newTrigger)
			}

			repeatTrigger := rp.ValidTrigger(testApp.ID, testFn.ID)
			repeatTrigger.Name = newTrigger.Name
			_, err = ds.InsertTrigger(ctx, repeatTrigger)
			if err != models.ErrTriggerExists {
				t.Errorf("Expected ErrTriggerExists, not %s", err)
			}
		})

		t.Run("valid trigger", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())
			testFn := h.GivenFnInDb(rp.ValidFn(testApp.ID))
			newTrigger := rp.ValidTrigger(testApp.ID, testFn.ID)
			insertedTrigger, err := ds.InsertTrigger(ctx, newTrigger)
			if err != nil {
				t.Fatalf("error when storing new trigger: %s", err)
			}
			if insertedTrigger.ID == "" {
				t.Fatalf("No ID ")
			}
			newTrigger.ID = insertedTrigger.ID
			if !newTrigger.EqualsWithAnnotationSubset(insertedTrigger) {
				t.Errorf("Expecting returned trigger %#v to equal %#v", insertedTrigger, newTrigger)
			}
		})

		t.Run("get trigger invalid ID", func(t *testing.T) {
			_, err := ds.GetTriggerByID(ctx, "notreal")
			if err != models.ErrTriggerNotFound {
				t.Fatalf("was expecting models.ErrTriggerNotFound : %s", err)
			}
		})

		t.Run("get existing trigger", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())
			testFn := h.GivenFnInDb(rp.ValidFn(testApp.ID))
			newTrigger := rp.ValidTrigger(testApp.ID, testFn.ID)
			insertedTrigger := h.GivenTriggerInDb(newTrigger)

			gotTrigger, err := ds.GetTriggerByID(ctx, insertedTrigger.ID)
			if err != nil {
				t.Fatalf("expecting no error, got: %s", err)
			}

			newTrigger.ID = insertedTrigger.ID
			if !newTrigger.EqualsWithAnnotationSubset(gotTrigger) {
				t.Errorf("Expecting returned trigger %#v to equal %#v", gotTrigger, newTrigger)
			}
		})

		t.Run("missing app Id", func(t *testing.T) {
			emptyFilter := &models.TriggerFilter{}
			_, err := ds.GetTriggers(ctx, emptyFilter)
			if err != models.ErrTriggerMissingAppID {
				t.Fatalf("expected models.ErrTriggerMissingAppID, but got %s", err)
			}
		})

		t.Run("non-existant app", func(t *testing.T) {
			nonMatchingFilter := &models.TriggerFilter{AppID: "notexist"}
			triggers, err := ds.GetTriggers(ctx, nonMatchingFilter)
			if len(triggers) != 0 && err == nil {
				t.Fatalf("expected empty trigger list and no error, but got list [%v] and err %s", triggers, err)
			}
		})

		t.Run("app id not same as fn id ", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp1 := h.GivenAppInDb(rp.ValidApp())
			testApp2 := h.GivenAppInDb(rp.ValidApp())
			testFn := h.GivenFnInDb(rp.ValidFn(testApp1.ID))

			tr := rp.ValidTrigger(testApp2.ID, testFn.ID)

			_, err := ds.InsertTrigger(ctx, tr)
			if err != models.ErrTriggerFnIDNotSameApp {
				t.Errorf("expected error when Fn ID did not match Trigger App ID, got %s", err)
			}
		})

		t.Run("filter triggers", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())
			testFn := h.GivenFnInDb(rp.ValidFn(testApp.ID))
			testFn2 := h.GivenFnInDb(rp.ValidFn(testApp.ID))

			var storedTriggers []*models.Trigger

			for i := 0; i < 10; i++ {
				trigger := rp.ValidTrigger(testApp.ID, testFn.ID)
				trigger.Source = fmt.Sprintf("src_%v", i)
				storedTriggers = append(storedTriggers, h.GivenTriggerInDb(trigger))
			}

			trigger := rp.ValidTrigger(testApp.ID, testFn2.ID)
			trigger.Source = fmt.Sprintf("src_%v", 11)
			h.GivenTriggerInDb(trigger)

			appIDFilter := &models.TriggerFilter{AppID: testApp.ID}
			triggers, err := ds.GetTriggers(ctx, appIDFilter)
			if err != nil {
				t.Fatalf("Test GetTriggers(get all triggers for app), not expecting err %s", err)
			}

			if len(triggers) != 11 {
				t.Fatalf("Test GetTriggers(get all triggers for app), expecting 10 results, got %d", len(triggers))
			}

			for i := 1; i < 10; i++ {
				if !storedTriggers[i].EqualsWithAnnotationSubset(triggers[i]) {
					t.Fatalf("expecting ordered by names, but aren't: %s, %s", storedTriggers[i].Name, triggers[i].Name)

				}
			}

			NameFilter := &models.TriggerFilter{AppID: testApp.ID, Name: storedTriggers[0].Name}
			triggers, err = ds.GetTriggers(ctx, NameFilter)
			if err != nil {
				t.Fatalf("Test GetTriggers(filter by name), not expecting err %s", err)
			}

			if len(triggers) != 1 {
				t.Fatalf("Test GetTriggers(filter by name), expecting 1 results, got %d", len(triggers))
			}

			if !storedTriggers[0].EqualsWithAnnotationSubset(triggers[0]) {
				t.Fatalf("expect single result to equal first stored result : %#v != %#v", triggers[4], storedTriggers[4])
			}

			appIDPagedFilter := &models.TriggerFilter{AppID: testApp.ID, PerPage: 5}
			triggers, err = ds.GetTriggers(ctx, appIDPagedFilter)
			if err != nil {
				t.Fatalf("Test GetTriggers(page triggers for app), not expecting err %s", err)
			}

			if len(triggers) != 5 {
				t.Fatalf("Test GetTriggers(get all triggers for app), expecting 5 results, got %d", len(triggers))
			}

			if !triggers[4].Equals(storedTriggers[4]) {
				t.Fatalf("expect 5th result to equal 5th stored result : %#v != %#v", triggers[4], storedTriggers[4])
			}

			appIDPagedFilter.Cursor = triggers[4].ID
			triggers, err = ds.GetTriggers(ctx, appIDPagedFilter)

			if err != nil {
				t.Fatalf("Test GetTriggers(page triggers for app), not expecting err %s", err)
			}

			if len(triggers) != 5 {
				t.Fatalf("Test GetTriggers(get all triggers for app), expecting 5 results, got %d", len(triggers))
			}

			if !storedTriggers[9].EqualsWithAnnotationSubset(triggers[4]) {
				t.Fatalf("expect 5th result to equal 9th stored result : %#v != %#v", triggers[4], storedTriggers[9])
			}

			// components are AND'd
			findNothingFilter := &models.TriggerFilter{AppID: testApp.ID, FnID: testFn.ID}
			triggers, err = ds.GetTriggers(ctx, findNothingFilter)
			if err != nil {
				t.Fatalf("Test GetTriggers(AND filtering), not expecting err %s", err)
			}
			if len(triggers) != 10 {
				t.Fatalf("Test GetTriggers(AND filtering), expecting 10 results, got %d", len(triggers))
			}
		})

		t.Run("update triggers", func(t *testing.T) {

			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())
			testFn := h.GivenFnInDb(rp.ValidFn(testApp.ID))
			testTrigger := h.GivenTriggerInDb(rp.ValidTrigger(testApp.ID, testFn.ID))

			testTrigger.Name = "newName"
			testTrigger.Source = "newSource"

			time.Sleep(10 * time.Millisecond)
			gotTrigger, err := ds.UpdateTrigger(ctx, testTrigger)
			if err != nil {
				t.Fatalf("error when updating trigger: %s", err)
			}

			if !testTrigger.EqualsWithAnnotationSubset(gotTrigger) {
				t.Fatalf("expecting returned triggers equal, got  : %#v : %#v", testTrigger, gotTrigger)
			}

			gotTrigger, err = ds.GetTriggerByID(ctx, testTrigger.ID)
			if err != nil {
				t.Fatalf("wasn't expecting an error : %s", err)
			}
			if !testTrigger.EqualsWithAnnotationSubset(gotTrigger) {
				t.Fatalf("expecting fetch trigger to be updated got  : %v : %v", testTrigger, gotTrigger)
			}

			if testTrigger.CreatedAt.String() != gotTrigger.CreatedAt.String() {
				t.Fatalf("create timestamps should match : %v : %v", testTrigger.CreatedAt, gotTrigger.CreatedAt)
			}

			if testTrigger.UpdatedAt.String() == gotTrigger.UpdatedAt.String() {
				t.Fatalf("update timestamps shouldn't match : %v : %v", testTrigger, gotTrigger)
			}

		})

		t.Run("remove non-existant", func(t *testing.T) {
			err := ds.RemoveTrigger(ctx, "nonexistant")

			if err != models.ErrTriggerNotFound {
				t.Fatalf("Expecting trigger not found , got %v ", err)
			}
		})

		t.Run("Remove existing", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())
			testFn := h.GivenFnInDb(rp.ValidFn(testApp.ID))
			testTrigger := h.GivenTriggerInDb(rp.ValidTrigger(testApp.ID, testFn.ID))
			err := ds.RemoveTrigger(ctx, testTrigger.ID)

			if err != nil {
				t.Fatalf("expecting no error, got %s", err)
			}

			_, err = ds.GetTriggerByID(ctx, testTrigger.ID)
			if err != models.ErrTriggerNotFound {
				t.Fatalf("was expecting ErrTriggerNotFound : %s", err)
			}
		})

		t.Run("Remove function should remove triggers", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())
			testFn := h.GivenFnInDb(rp.ValidFn(testApp.ID))
			testTrigger := h.GivenTriggerInDb(rp.ValidTrigger(testApp.ID, testFn.ID))
			err := ds.RemoveFn(ctx, testFn.ID)
			if err != nil {
				t.Fatalf("expecting no error, got %s", err)
			}

			tr, err := ds.GetTriggerByID(ctx, testTrigger.ID)
			if err != models.ErrTriggerNotFound {
				t.Fatalf("was expecting ErrTriggerNotFound got %s %#v", err, tr)
			}
		})

		t.Run("Remove app should remove triggers", func(t *testing.T) {
			h := NewHarness(t, ctx, ds)
			defer h.Cleanup()
			testApp := h.GivenAppInDb(rp.ValidApp())
			testFn := h.GivenFnInDb(rp.ValidFn(testApp.ID))
			testTrigger := h.GivenTriggerInDb(rp.ValidTrigger(testApp.ID, testFn.ID))
			err := ds.RemoveApp(ctx, testFn.AppID)
			if err != nil {
				t.Fatalf("expecting no error, got %s", err)
			}

			tr, err := ds.GetTriggerByID(ctx, testTrigger.ID)
			if err != models.ErrTriggerNotFound {
				t.Fatalf("was expecting ErrTriggerNotFound got %s %#v", err, tr)
			}
		})

	})
}

func RunAllTests(t *testing.T, dsf DataStoreFunc, rp ResourceProvider) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()

	RunAppsTest(t, dsf, rp)
	RunRoutesTest(t, dsf, rp)
	RunFnsTest(t, dsf, rp)
	RunTriggersTest(t, dsf, rp)

}
