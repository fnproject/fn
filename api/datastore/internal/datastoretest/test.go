package datastoretest

import (
	"bytes"
	"context"
	"log"
	"testing"
	"time"

	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
	"github.com/go-openapi/strfmt"
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

func Test(t *testing.T, dsf func(t *testing.T) models.Datastore) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()

	testApp.SetDefaults()
	testRoute.AppID = testApp.ID

	ctx := context.Background()

	call := new(models.Call)
	call.CreatedAt = strfmt.DateTime(time.Now())
	call.Status = "error"
	call.Error = "ya dun goofed"
	call.StartedAt = strfmt.DateTime(time.Now())
	call.CompletedAt = strfmt.DateTime(time.Now())
	call.AppName = testApp.Name
	call.Path = testRoute.Path

	t.Run("call-insert", func(t *testing.T) {
		ds := dsf(t)
		call.ID = id.New().String()
		err := ds.InsertCall(ctx, call)
		if err != nil {
			t.Fatalf("Test InsertCall(ctx, &call): unexpected error `%v`", err)
		}
	})

	t.Run("call-atomic-update", func(t *testing.T) {
		ds := dsf(t)
		call.ID = id.New().String()
		err := ds.InsertCall(ctx, call)
		if err != nil {
			t.Fatalf("Test UpdateCall: unexpected error `%v`", err)
		}
		newCall := new(models.Call)
		*newCall = *call
		newCall.Status = "success"
		newCall.Error = ""
		err = ds.UpdateCall(ctx, call, newCall)
		if err != nil {
			t.Fatalf("Test UpdateCall: unexpected error `%v`", err)
		}
		dbCall, err := ds.GetCall(ctx, call.AppName, call.ID)
		if err != nil {
			t.Fatalf("Test UpdateCall: unexpected error `%v`", err)
		}
		if dbCall.ID != newCall.ID {
			t.Fatalf("Test GetCall: id mismatch `%v` `%v`", call.ID, newCall.ID)
		}
		if dbCall.Status != newCall.Status {
			t.Fatalf("Test GetCall: status mismatch `%v` `%v`", call.Status, newCall.Status)
		}
		if dbCall.Error != newCall.Error {
			t.Fatalf("Test GetCall: error mismatch `%v` `%v`", call.Error, newCall.Error)
		}
		if time.Time(dbCall.CreatedAt).Unix() != time.Time(newCall.CreatedAt).Unix() {
			t.Fatalf("Test GetCall: created_at mismatch `%v` `%v`", call.CreatedAt, newCall.CreatedAt)
		}
		if time.Time(dbCall.StartedAt).Unix() != time.Time(newCall.StartedAt).Unix() {
			t.Fatalf("Test GetCall: started_at mismatch `%v` `%v`", call.StartedAt, newCall.StartedAt)
		}
		if time.Time(dbCall.CompletedAt).Unix() != time.Time(newCall.CompletedAt).Unix() {
			t.Fatalf("Test GetCall: completed_at mismatch `%v` `%v`", call.CompletedAt, newCall.CompletedAt)
		}
		if dbCall.AppName != newCall.AppName {
			t.Fatalf("Test GetCall: app_name mismatch `%v` `%v`", call.AppName, newCall.AppName)
		}
		if dbCall.Path != newCall.Path {
			t.Fatalf("Test GetCall: path mismatch `%v` `%v`", call.Path, newCall.Path)
		}
	})

	t.Run("call-atomic-update-no-existing-call", func(t *testing.T) {
		ds := dsf(t)
		call.ID = id.New().String()
		// Do NOT insert the call
		newCall := new(models.Call)
		*newCall = *call
		newCall.Status = "success"
		newCall.Error = ""
		err := ds.UpdateCall(ctx, call, newCall)
		if err != models.ErrCallNotFound {
			t.Fatalf("Test UpdateCall: unexpected error `%v`", err)
		}
	})

	t.Run("call-atomic-update-unexpected-existing-call", func(t *testing.T) {
		ds := dsf(t)
		call.ID = id.New().String()
		err := ds.InsertCall(ctx, call)
		if err != nil {
			t.Fatalf("Test UpdateCall: unexpected error `%v`", err)
		}
		// Now change the 'from' call so it becomes different from the db
		badFrom := new(models.Call)
		*badFrom = *call
		badFrom.Status = "running"
		newCall := new(models.Call)
		*newCall = *call
		newCall.Status = "success"
		newCall.Error = ""
		err = ds.UpdateCall(ctx, badFrom, newCall)
		if err != models.ErrDatastoreCannotUpdateCall {
			t.Fatalf("Test UpdateCall: unexpected error `%v`", err)
		}
	})

	t.Run("call-get", func(t *testing.T) {
		ds := dsf(t)
		call.ID = id.New().String()
		err := ds.InsertCall(ctx, call)
		if err != nil {
			t.Fatalf("Test GetCall: unexpected error `%v`", err)
		}
		newCall, err := ds.GetCall(ctx, call.AppName, call.ID)
		if err != nil {
			t.Fatalf("Test GetCall: unexpected error `%v`", err)
		}
		if call.ID != newCall.ID {
			t.Fatalf("Test GetCall: id mismatch `%v` `%v`", call.ID, newCall.ID)
		}
		if call.Status != newCall.Status {
			t.Fatalf("Test GetCall: status mismatch `%v` `%v`", call.Status, newCall.Status)
		}
		if call.Error != newCall.Error {
			t.Fatalf("Test GetCall: error mismatch `%v` `%v`", call.Error, newCall.Error)
		}
		if time.Time(call.CreatedAt).Unix() != time.Time(newCall.CreatedAt).Unix() {
			t.Fatalf("Test GetCall: created_at mismatch `%v` `%v`", call.CreatedAt, newCall.CreatedAt)
		}
		if time.Time(call.StartedAt).Unix() != time.Time(newCall.StartedAt).Unix() {
			t.Fatalf("Test GetCall: started_at mismatch `%v` `%v`", call.StartedAt, newCall.StartedAt)
		}
		if time.Time(call.CompletedAt).Unix() != time.Time(newCall.CompletedAt).Unix() {
			t.Fatalf("Test GetCall: completed_at mismatch `%v` `%v`", call.CompletedAt, newCall.CompletedAt)
		}
		if call.AppName != newCall.AppName {
			t.Fatalf("Test GetCall: app_name mismatch `%v` `%v`", call.AppName, newCall.AppName)
		}
		if call.Path != newCall.Path {
			t.Fatalf("Test GetCall: path mismatch `%v` `%v`", call.Path, newCall.Path)
		}
	})

	t.Run("calls-get", func(t *testing.T) {
		ds := dsf(t)
		filter := &models.CallFilter{AppName: call.AppName, Path: call.Path, PerPage: 100}
		call.ID = id.New().String()
		call.CreatedAt = strfmt.DateTime(time.Now())
		err := ds.InsertCall(ctx, call)
		if err != nil {
			t.Fatal(err)
		}
		calls, err := ds.GetCalls(ctx, filter)
		if err != nil {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected error `%v`", err)
		}
		if len(calls) != 1 {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected length `%v`", len(calls))
		}

		c2 := *call
		c3 := *call
		c2.ID = id.New().String()
		c2.CreatedAt = strfmt.DateTime(time.Now().Add(100 * time.Millisecond)) // add ms cuz db uses it for sort
		c3.ID = id.New().String()
		c3.CreatedAt = strfmt.DateTime(time.Now().Add(200 * time.Millisecond))

		err = ds.InsertCall(ctx, &c2)
		if err != nil {
			t.Fatal(err)
		}
		err = ds.InsertCall(ctx, &c3)
		if err != nil {
			t.Fatal(err)
		}

		// test that no filter works too
		calls, err = ds.GetCalls(ctx, &models.CallFilter{PerPage: 100})
		if err != nil {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected error `%v`", err)
		}
		if len(calls) != 3 {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected length `%v`", len(calls))
		}

		// test that pagination stuff works. id, descending
		filter.PerPage = 1
		calls, err = ds.GetCalls(ctx, filter)
		if err != nil {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected error `%v`", err)
		}
		if len(calls) != 1 {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected length `%v`", len(calls))
		} else if calls[0].ID != c3.ID {
			t.Fatalf("Test GetCalls: call ids not in expected order: %v %v", calls[0].ID, c3.ID)
		}

		filter.PerPage = 100
		filter.Cursor = calls[0].ID
		calls, err = ds.GetCalls(ctx, filter)
		if err != nil {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected error `%v`", err)
		}
		if len(calls) != 2 {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected length `%v`", len(calls))
		} else if calls[0].ID != c2.ID {
			t.Fatalf("Test GetCalls: call ids not in expected order: %v %v", calls[0].ID, c2.ID)
		} else if calls[1].ID != call.ID {
			t.Fatalf("Test GetCalls: call ids not in expected order: %v %v", calls[1].ID, call.ID)
		}

		// test that filters actually applied
		calls, err = ds.GetCalls(ctx, &models.CallFilter{AppName: "wrongappname", PerPage: 100})
		if err != nil {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected error `%v`", err)
		}
		if len(calls) != 0 {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected length `%v`", len(calls))
		}

		calls, err = ds.GetCalls(ctx, &models.CallFilter{Path: "wrongpath", PerPage: 100})
		if err != nil {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected error `%v`", err)
		}
		if len(calls) != 0 {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected length `%v`", len(calls))
		}

		// make sure from_time and to_time work
		filter = &models.CallFilter{
			PerPage:  100,
			FromTime: call.CreatedAt,
			ToTime:   c3.CreatedAt,
		}
		calls, err = ds.GetCalls(ctx, filter)
		if err != nil {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected error `%v`", err)
		}
		if len(calls) != 1 {
			t.Fatalf("Test GetCalls(ctx, filter): unexpected length `%v`", len(calls))
		} else if calls[0].ID != c2.ID {
			t.Fatalf("Test GetCalls: call id not expected %s vs %s", calls[0].ID, c2.ID)
		}
	})

	t.Run("apps", func(t *testing.T) {
		ds := dsf(t)
		// Testing insert app
		_, err := ds.InsertApp(ctx, nil)
		if err != models.ErrDatastoreEmptyApp {
			t.Fatalf("Test InsertApp(nil): expected error `%v`, but it was `%v`", models.ErrDatastoreEmptyApp, err)
		}

		_, err = ds.InsertApp(ctx, &models.App{})
		if err != models.ErrAppsMissingName {
			t.Fatalf("Test InsertApp(&{}): expected error `%v`, but it was `%v`", models.ErrAppsMissingName, err)
		}

		inserted, err := ds.InsertApp(ctx, testApp)
		if err != nil {
			t.Fatalf("Test InsertApp: error when storing new app: %s", err)
		}
		if !inserted.Equals(testApp) {
			t.Fatalf("Test InsertApp: expected to insert:\n%v\nbut got:\n%v", testApp, inserted)
		}

		_, err = ds.InsertApp(ctx, testApp)
		if err != models.ErrAppsAlreadyExists {
			t.Fatalf("Test InsertApp duplicated: expected error `%v`, but it was `%v`", models.ErrAppsAlreadyExists, err)
		}

		{
			// Set a config var
			updated, err := ds.UpdateApp(ctx, &models.App{Name: testApp.Name, Config: map[string]string{"TEST": "1"}})
			if err != nil {
				t.Fatalf("Test UpdateApp: error when updating app: %v", err)
			}
			expected := &models.App{Name: testApp.Name, Config: map[string]string{"TEST": "1"}}
			if !updated.Equals(expected) {
				t.Fatalf("Test UpdateApp: expected updated `%v` but got `%v`", expected, updated)
			}

			// Set a different var (without clearing the existing)
			updated, err = ds.UpdateApp(ctx,
				&models.App{Name: testApp.Name, Config: map[string]string{"OTHER": "TEST"}})
			if err != nil {
				t.Fatalf("Test UpdateApp: error when updating app: %v", err)
			}
			expected = &models.App{Name: testApp.Name, Config: map[string]string{"TEST": "1", "OTHER": "TEST"}}
			if !updated.Equals(expected) {
				t.Fatalf("Test UpdateApp: expected updated `%v` but got `%v`", expected, updated)
			}

			// Delete a var
			updated, err = ds.UpdateApp(ctx,
				&models.App{Name: testApp.Name, Config: map[string]string{"TEST": ""}})
			if err != nil {
				t.Fatalf("Test UpdateApp: error when updating app: %v", err)
			}
			expected = &models.App{Name: testApp.Name, Config: map[string]string{"OTHER": "TEST"}}
			if !updated.Equals(expected) {
				t.Fatalf("Test UpdateApp: expected updated `%v` but got `%v`", expected, updated)
			}
		}

		// Testing get app
		_, err = ds.GetApp(ctx, "")
		if err != models.ErrAppsMissingName {
			t.Fatalf("Test GetApp: expected error to be %v, but it was %s", models.ErrAppsMissingName, err)
		}

		app, err := ds.GetApp(ctx, testApp.Name)
		if err != nil {
			t.Fatalf("Test GetApp: error: %s", err)
		}
		if app.Name != testApp.Name {
			t.Fatalf("Test GetApp: expected `app.Name` to be `%s` but it was `%s`", app.Name, testApp.Name)
		}

		// Testing list apps
		apps, err := ds.GetApps(ctx, &models.AppFilter{PerPage: 100})
		if err != nil {
			t.Fatalf("Test GetApps: unexpected error %v", err)
		}
		if len(apps) == 0 {
			t.Fatal("Test GetApps: expected result count to be greater than 0")
		}
		if apps[0].Name != testApp.Name {
			t.Fatalf("Test GetApps: expected `app.Name` to be `%s` but it was `%s`", app.Name, testApp.Name)
		}

		// test pagination stuff (ordering / limits / cursoring)
		a2 := *testApp
		a3 := *testApp
		a2.Name = "Testa"
		a2.SetDefaults()
		a3.Name = "Testb"
		a3.SetDefaults()
		if _, err = ds.InsertApp(ctx, &a2); err != nil {
			t.Fatal(err)
		}
		if _, err = ds.InsertApp(ctx, &a3); err != nil {
			t.Fatal(err)
		}

		apps, err = ds.GetApps(ctx, &models.AppFilter{PerPage: 1})
		if err != nil {
			t.Fatalf("Test GetApps: error: %s", err)
		}
		if len(apps) != 1 {
			t.Fatalf("Test GetApps: expected result count to be 1 but got %d", len(apps))
		} else if apps[0].Name != testApp.Name {
			t.Fatalf("Test GetApps: expected `app.Name` to be `%s` but it was `%s`", testApp.Name, apps[0].Name)
		}

		apps, err = ds.GetApps(ctx, &models.AppFilter{PerPage: 100, Cursor: apps[0].Name})
		if err != nil {
			t.Fatalf("Test GetApps: error: %s", err)
		}
		if len(apps) != 2 {
			t.Fatalf("Test GetApps: expected result count to be 2 but got %d", len(apps))
		} else if apps[0].Name != a2.Name {
			t.Fatalf("Test GetApps: expected `app.Name` to be `%s` but it was `%s`", a2.Name, apps[0].Name)
		} else if apps[1].Name != a3.Name {
			t.Fatalf("Test GetApps: expected `app.Name` to be `%s` but it was `%s`", a3.Name, apps[1].Name)
		}

		a4 := *testApp
		a4.Name = "Abcdefg" // < /test lexicographically, but not in length
		a4.SetDefaults()
		if _, err = ds.InsertApp(ctx, &a4); err != nil {
			t.Fatal(err)
		}

		apps, err = ds.GetApps(ctx, &models.AppFilter{PerPage: 100})
		if err != nil {
			t.Fatalf("Test GetApps: error: %s", err)
		}
		if len(apps) != 4 {
			t.Fatalf("Test GetApps: expected result count to be 4 but got %d", len(apps))
		} else if apps[0].Name != a4.Name {
			t.Fatalf("Test GetApps: expected `app.Name` to be `%s` but it was `%s`", a4.Name, apps[0].Name)
		}

		// TODO fix up prefix stuff
		//apps, err = ds.GetApps(ctx, &models.AppFilter{Name: "Tes"})
		//if err != nil {
		//t.Fatalf("Test GetApps(filter): unexpected error %v", err)
		//}
		//if len(apps) != 3 {
		//t.Fatal("Test GetApps(filter): expected result count to be 3, got", len(apps))
		//}

		// Testing app delete
		err = ds.RemoveApp(ctx, "")
		if err != models.ErrAppsMissingName {
			t.Fatalf("Test RemoveApp: expected error `%v`, but it was `%v`", models.ErrAppsMissingName, err)
		}

		err = ds.RemoveApp(ctx, testApp.Name)
		if err != nil {
			t.Fatalf("Test RemoveApp: error: %s", err)
		}
		app, err = ds.GetApp(ctx, testApp.Name)
		if err != models.ErrAppsNotFound {
			t.Fatalf("Test GetApp(removed): expected error `%v`, but it was `%v`", models.ErrAppsNotFound, err)
		}
		if app != nil {
			t.Log(err.Error())
			t.Fatal("Test RemoveApp: failed to remove the app, app should be gone already")
		}

		// Test update inexistent app
		_, err = ds.UpdateApp(ctx, &models.App{
			Name: testApp.Name,
			Config: map[string]string{
				"TEST": "1",
			},
		})
		if err != models.ErrAppsNotFound {
			t.Fatalf("Test UpdateApp(inexistent): expected error `%v`, but it was `%v`", models.ErrAppsNotFound, err)
		}
	})

	t.Run("routes", func(t *testing.T) {
		ds := dsf(t)
		// Insert app again to test routes
		_, err := ds.InsertApp(ctx, testApp)
		if err != nil && err != models.ErrAppsAlreadyExists {
			t.Fatal("Test InsertRoute Prep: failed to insert app: ", err)
		}

		// Testing insert route
		{
			_, err = ds.InsertRoute(ctx, nil)
			if err != models.ErrDatastoreEmptyRoute {
				t.Fatalf("Test InsertRoute(nil): expected error `%v`, but it was `%v`", models.ErrDatastoreEmptyRoute, err)
			}

			_, err = ds.InsertRoute(ctx, &models.Route{AppID: "notreal", Path: "/test"})
			if err != models.ErrAppsNotFound {
				t.Fatalf("Test InsertRoute: expected error `%v`, but it was `%v`", models.ErrAppsNotFound, err)
			}

			_, err = ds.InsertRoute(ctx, testRoute)
			if err != nil {
				t.Fatalf("Test InsertRoute: error when storing new route: %s", err)
			}

			_, err = ds.InsertRoute(ctx, testRoute)
			if err != models.ErrRoutesAlreadyExists {
				t.Fatalf("Test InsertRoute duplicated: expected error to be `%v`, but it was `%v`", models.ErrRoutesAlreadyExists, err)
			}
		}

		// Testing get
		{
			_, err = ds.GetRoute(ctx, "a", "")
			if err != models.ErrRoutesMissingPath {
				t.Fatalf("Test GetRoute(empty route path): expected error `%v`, but it was `%v`", models.ErrRoutesMissingPath, err)
			}

			_, err = ds.GetRoute(ctx, "", "a")
			if err != models.ErrAppsMissingName {
				t.Fatalf("Test GetRoute(empty app name): expected error `%v`, but it was `%v`", models.ErrAppsMissingName, err)
			}

			route, err := ds.GetRoute(ctx, testApp.Name, testRoute.Path)
			if err != nil {
				t.Fatalf("Test GetRoute: unexpected error %v", err)
			}
			if !route.Equals(testRoute) {
				t.Fatalf("Test InsertApp: expected to insert:\n%v\nbut got:\n%v", testRoute, *route)
			}
		}

		// Testing update
		{
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
				t.Fatalf("Test UpdateRoute: unexpected error: %v", err)
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
				t.Fatalf("Test UpdateRoute: expected updated `%v` but got `%v`", expected, updated)
			}

			// Update a config var, remove another. Add one Header, remove another.
			updated, err = ds.UpdateRoute(ctx, &models.Route{
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
				t.Fatalf("Test UpdateRoute: unexpected error: %v", err)
			}
			expected = &models.Route{
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
				t.Fatalf("Test UpdateRoute: expected updated:\n`%v`\nbut got:\n`%v`", expected, updated)
			}
		}

		// Testing list routes
		routes, err := ds.GetRoutesByApp(ctx, testApp.Name, &models.RouteFilter{PerPage: 1})
		if err != nil {
			t.Fatalf("Test GetRoutesByApp: unexpected error %v", err)
		}
		if len(routes) == 0 {
			t.Fatal("Test GetRoutesByApp: expected result count to be greater than 0")
		}
		if routes[0] == nil {
			t.Fatalf("Test GetRoutes: expected non-nil route")
		} else if routes[0].Path != testRoute.Path {
			t.Fatalf("Test GetRoutes: expected `app.Name` to be `%s` but it was `%s`", testRoute.Path, routes[0].Path)
		}

		routes, err = ds.GetRoutesByApp(ctx, testApp.Name, &models.RouteFilter{Image: testRoute.Image, PerPage: 1})
		if err != nil {
			t.Fatalf("Test GetRoutesByApp: unexpected error %v", err)
		}
		if len(routes) == 0 {
			t.Fatal("Test GetRoutesByApp: expected result count to be greater than 0")
		}
		if routes[0] == nil {
			t.Fatalf("Test GetRoutesByApp: expected non-nil route")
		} else if routes[0].Path != testRoute.Path {
			t.Fatalf("Test GetRoutesByApp: expected `route.Path` to be `%s` but it was `%s`", testRoute.Path, routes[0].Path)
		}

		routes, err = ds.GetRoutesByApp(ctx, "notreal", &models.RouteFilter{PerPage: 1})
		if err != nil {
			t.Fatalf("Test GetRoutesByApp: error: %s", err)
		}
		if len(routes) != 0 {
			t.Fatalf("Test GetRoutesByApp: expected result count to be 0 but got %d", len(routes))
		}

		// test pagination stuff
		r2 := *testRoute
		r2.AppID = testApp.ID
		r3 := *testRoute
		r2.AppID = testApp.ID
		r2.Path = "/testa"
		r3.Path = "/testb"

		if _, err = ds.InsertRoute(ctx, &r2); err != nil {
			t.Fatal(err)
		}
		if _, err = ds.InsertRoute(ctx, &r3); err != nil {
			t.Fatal(err)
		}

		routes, err = ds.GetRoutesByApp(ctx, testApp.Name, &models.RouteFilter{PerPage: 1})
		if err != nil {
			t.Fatalf("Test GetRoutesByApp: error: %s", err)
		}
		if len(routes) != 1 {
			t.Fatalf("Test GetRoutesByApp: expected result count to be 1 but got %d", len(routes))
		} else if routes[0].Path != testRoute.Path {
			t.Fatalf("Test GetRoutesByApp: expected `route.Path` to be `%s` but it was `%s`", testRoute.Path, routes[0].Path)
		}

		routes, err = ds.GetRoutesByApp(ctx, testApp.Name, &models.RouteFilter{PerPage: 2, Cursor: routes[0].Path})
		if err != nil {
			t.Fatalf("Test GetRoutesByApp: error: %s", err)
		}
		if len(routes) != 2 {
			t.Fatalf("Test GetRoutesByApp: expected result count to be 2 but got %d", len(routes))
		} else if routes[0].Path != r2.Path {
			t.Fatalf("Test GetRoutesByApp: expected `route.Path` to be `%s` but it was `%s`", r2.Path, routes[0].Path)
		} else if routes[1].Path != r3.Path {
			t.Fatalf("Test GetRoutesByApp: expected `route.Path` to be `%s` but it was `%s`", r3.Path, routes[1].Path)
		}

		r4 := *testRoute
		r4.Path = "/abcdefg" // < /test lexicographically, but not in length

		if _, err = ds.InsertRoute(ctx, &r4); err != nil {
			t.Fatal(err)
		}

		routes, err = ds.GetRoutesByApp(ctx, testApp.Name, &models.RouteFilter{PerPage: 100})
		if err != nil {
			t.Fatalf("Test GetRoutesByApp: error: %s", err)
		}
		if len(routes) != 4 {
			t.Fatalf("Test GetRoutesByApp: expected result count to be 4 but got %d", len(routes))
		} else if routes[0].Path != r4.Path {
			t.Fatalf("Test GetRoutesByApp: expected `route.Path` to be `%s` but it was `%s`", r4.Path, routes[0].Path)
		}

		// TODO test weird ordering possibilities ?
		// TODO test prefix filtering

		// Testing route delete
		err = ds.RemoveRoute(ctx, "", "")
		if err != models.ErrAppsMissingName {
			t.Fatalf("Test RemoveRoute(empty app name): expected error `%v`, but it was `%v`", models.ErrAppsMissingName, err)
		}

		err = ds.RemoveRoute(ctx, "a", "")
		if err != models.ErrRoutesMissingPath {
			t.Fatalf("Test RemoveRoute(empty route path): expected error `%v`, but it was `%v`", models.ErrRoutesMissingPath, err)
		}

		err = ds.RemoveRoute(ctx, testRoute.AppID, testRoute.Path)
		if err != nil {
			t.Fatalf("Test RemoveApp: unexpected error: %v", err)
		}

		route, err := ds.GetRoute(ctx, testApp.Name, testRoute.Path)
		if err != nil && err != models.ErrRoutesNotFound {
			t.Fatalf("Test GetRoute: expected error `%v`, but it was `%v`", models.ErrRoutesNotFound, err)
		}
		if route != nil {
			t.Fatalf("Test RemoveApp: failed to remove the route: %v", route)
		}

		_, err = ds.UpdateRoute(ctx, &models.Route{
			AppID: testApp.ID,
			Path:  testRoute.Path,
			Image: "test",
		})
		if err != models.ErrRoutesNotFound {
			t.Fatalf("Test UpdateRoute inexistent: expected error to be `%v`, but it was `%v`", models.ErrRoutesNotFound, err)
		}
	})
}

var testApp = &models.App{
	Name: "Test",
}

var testRoute = &models.Route{
	Path:        "/test",
	Image:       "fnproject/fn-test-utils",
	Type:        "sync",
	Format:      "http",
	Timeout:     models.DefaultTimeout,
	IdleTimeout: models.DefaultIdleTimeout,
	Memory:      models.DefaultMemory,
}
