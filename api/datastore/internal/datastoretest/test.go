package datastoretest

import (
	"bytes"
	"context"
	"log"
	"testing"

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
		testApp.ID = inserted.ID

		{
			// Set a config var
			testApp, err := ds.GetAppByID(ctx, testApp.ID)
			if err != nil {
				t.Fatal(err.Error())
			}
			testApp.Config = map[string]string{"TEST": "1"}
			updated, err := ds.UpdateApp(ctx, testApp)
			if err != nil {
				t.Fatalf("Test UpdateApp: error when updating app: %v", err)
			}
			expected := &models.App{ID: testApp.ID, Name: testApp.Name, Config: map[string]string{"TEST": "1"}}
			if !updated.Equals(expected) {
				t.Fatalf("Test UpdateApp: expected updated `%v` but got `%v`", expected, updated)
			}

			// Set a different var (without clearing the existing)
			another := testApp.Clone()
			another.Config = map[string]string{"OTHER": "TEST"}
			updated, err = ds.UpdateApp(ctx, another)
			if err != nil {
				t.Fatalf("Test UpdateApp: error when updating app: %v", err)
			}
			expected = &models.App{Name: testApp.Name, ID: testApp.ID, Config: map[string]string{"TEST": "1", "OTHER": "TEST"}}
			if !updated.Equals(expected) {
				t.Fatalf("Test UpdateApp: expected updated `%v` but got `%v`", expected, updated)
			}

			// Delete a var
			dVar := testApp.Clone()
			dVar.Config = map[string]string{"TEST": ""}
			updated, err = ds.UpdateApp(ctx, dVar)
			if err != nil {
				t.Fatalf("Test UpdateApp: error when updating app: %v", err)
			}
			expected = &models.App{Name: testApp.Name, ID: testApp.ID, Config: map[string]string{"OTHER": "TEST"}}
			if !updated.Equals(expected) {
				t.Fatalf("Test UpdateApp: expected updated `%v` but got `%v`", expected, updated)
			}
		}

		// Testing get app
		_, err = ds.GetAppByID(ctx, "")
		if err != models.ErrDatastoreEmptyAppID {
			t.Fatalf("Test GetApp: expected error to be %v, but it was %s", models.ErrDatastoreEmptyAppID, err)
		}

		app, err := ds.GetAppByID(ctx, testApp.ID)
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
		a2 := &models.App{Name: "Testa"}
		a2.SetDefaults()
		a3 := &models.App{Name: "Testb"}
		a3.SetDefaults()
		if _, err = ds.InsertApp(ctx, a2); err != nil {
			t.Fatal(err)
		}
		if _, err = ds.InsertApp(ctx, a3); err != nil {
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

		a4 := &models.App{Name: "Abcdefg"}
		a4.SetDefaults()
		if _, err = ds.InsertApp(ctx, a4); err != nil {
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
		if err != models.ErrDatastoreEmptyAppID {
			t.Fatalf("Test RemoveApp: expected error `%v`, but it was `%v`", models.ErrDatastoreEmptyAppID, err)
		}

		testApp, _ := ds.GetAppByID(ctx, testApp.ID)
		err = ds.RemoveApp(ctx, testApp.ID)
		if err != nil {
			t.Fatalf("Test RemoveApp: error: %s", err)
		}
		app, err = ds.GetAppByID(ctx, testApp.ID)
		if err != models.ErrAppsNotFound {
			t.Fatalf("Test GetApp(removed): expected error `%v`, but it was `%v`", models.ErrAppsNotFound, err)
		}
		if app != nil {
			t.Log(err.Error())
			t.Fatal("Test RemoveApp: failed to remove the app, app should be gone already")
		}

		// Test update inexistent app
		missingApp := &models.App{
			Name: testApp.Name,
			Config: map[string]string{
				"TEST": "1",
			},
		}
		missingApp.SetDefaults()
		_, err = ds.UpdateApp(ctx, missingApp)
		if err != models.ErrAppsNotFound {
			t.Fatalf("Test UpdateApp(inexistent): expected error `%v`, but it was `%v`", models.ErrAppsNotFound, err)
		}
	})

	t.Run("routes", func(t *testing.T) {
		ds := dsf(t)
		// Insert app again to test fns
		testApp, err := ds.InsertApp(ctx, testApp)
		if err != nil && err != models.ErrAppsAlreadyExists {
			t.Fatal("Test InsertRoute Prep: failed to insert app: ", err)
		}

		// Testing insert route
		{
			_, err = ds.InsertRoute(ctx, nil)
			if err != models.ErrDatastoreEmptyRoute {
				t.Fatalf("Test InsertRoute(nil): expected error `%v`, but it was `%v`", models.ErrDatastoreEmptyRoute, err)
			}

			newTestRoute := testRoute.Clone()
			newTestRoute.AppID = "notreal"
			_, err = ds.InsertRoute(ctx, newTestRoute)
			if err != models.ErrAppsNotFound {
				t.Fatalf("Test InsertRoute: expected error `%v`, but it was `%v`", models.ErrAppsNotFound, err)
			}

			testRoute.AppID = testApp.ID
			testRoute, err = ds.InsertRoute(ctx, testRoute)
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
			_, err = ds.GetRoute(ctx, id.New().String(), "")
			if err != models.ErrRoutesMissingPath {
				t.Fatalf("Test GetRoute(empty route path): expected error `%v`, but it was `%v`", models.ErrRoutesMissingPath, err)
			}

			_, err = ds.GetRoute(ctx, "", "a")
			if err != models.ErrDatastoreEmptyAppID {
				t.Fatalf("Test GetRoute(empty app name): expected error `%v`, but it was `%v`", models.ErrRoutesMissingPath, err)
			}

			route, err := ds.GetRoute(ctx, testApp.ID, testRoute.Path)
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

		// Testing list fns
		routes, err := ds.GetRoutesByApp(ctx, testApp.ID, &models.RouteFilter{PerPage: 1})
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

		routes, err = ds.GetRoutesByApp(ctx, testApp.ID, &models.RouteFilter{Image: testRoute.Image, PerPage: 1})
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

		nre := &models.App{Name: "notreal"}
		nre.SetDefaults()
		routes, err = ds.GetRoutesByApp(ctx, nre.ID, &models.RouteFilter{PerPage: 1})
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

		routes, err = ds.GetRoutesByApp(ctx, testApp.ID, &models.RouteFilter{PerPage: 1})
		if err != nil {
			t.Fatalf("Test GetRoutesByApp: error: %s", err)
		}
		if len(routes) != 1 {
			t.Fatalf("Test GetRoutesByApp: expected result count to be 1 but got %d", len(routes))
		} else if routes[0].Path != testRoute.Path {
			t.Fatalf("Test GetRoutesByApp: expected `route.Path` to be `%s` but it was `%s`", testRoute.Path, routes[0].Path)
		}

		routes, err = ds.GetRoutesByApp(ctx, testApp.ID, &models.RouteFilter{PerPage: 2, Cursor: routes[0].Path})
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

		routes, err = ds.GetRoutesByApp(ctx, testApp.ID, &models.RouteFilter{PerPage: 100})
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
		if err != models.ErrDatastoreEmptyAppID {
			t.Fatalf("Test RemoveRoute(empty app name): expected error `%v`, but it was `%v`", models.ErrDatastoreEmptyAppID, err)
		}

		err = ds.RemoveRoute(ctx, testApp.ID, "")
		if err != models.ErrRoutesMissingPath {
			t.Fatalf("Test RemoveRoute(empty route path): expected error `%v`, but it was `%v`", models.ErrRoutesMissingPath, err)
		}

		err = ds.RemoveRoute(ctx, testApp.ID, testRoute.Path)
		if err != nil {
			t.Fatalf("Test RemoveApp: unexpected error: %v", err)
		}

		route, err := ds.GetRoute(ctx, testApp.ID, testRoute.Path)
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

	t.Run("fns", func(t *testing.T) {
		ds := dsf(t)
		// Testing insert func
		{
			_, err := ds.PutFn(ctx, nil)
			if err != models.ErrDatastoreEmptyFn {
				t.Fatalf("Test PutFn(nil): expected error `%v`, but it was `%v`", models.ErrDatastoreEmptyFn, err)
			}

			testFn2 := *testFn
			testFn2.Name = ""
			_, err = ds.PutFn(ctx, &testFn2)
			if err != models.ErrDatastoreEmptyFnName {
				t.Fatalf("Test PutFn(empty func name): expected error `%v`, but it was `%v`", models.ErrDatastoreEmptyFnName, err)
			}

			// assign to make sure that PutFn returns the right thing
			testFn, err = ds.PutFn(ctx, testFn)
			if err != nil {
				t.Fatalf("Test PutFn: error when storing perfectly good func: %s", err)
			}
		}

		// Testing get
		{
			_, err := ds.GetFn(ctx, "")
			if err != models.ErrDatastoreEmptyFnName {
				t.Fatalf("Test GetRoute(empty func path): expected error `%v`, but it was `%v`", models.ErrDatastoreEmptyFnName, err)
			}

			fn, err := ds.GetFn(ctx, testFn.Name)
			if err != nil {
				t.Fatalf("Test GetFn: unexpected error %v", err)
			}
			if !fn.Equals(testFn) {
				t.Fatalf("Test GetFn: expected to get the right func:\n%v\nbut got:\n%v", testFn, fn)
			}
		}

		// Testing update
		{
			// Update some fields, and add 3 configs
			updated, err := ds.PutFn(ctx, &models.Fn{
				Name: testFn.Name,
				Config: map[string]string{
					"FIRST":  "1",
					"SECOND": "2",
					"THIRD":  "3",
				},
			})
			if err != nil {
				t.Fatalf("Test UpdateRoute: unexpected error: %v", err)
			}
			expected := &models.Fn{
				// unchanged
				ID:     testFn.ID,
				Name:   testFn.Name,
				Image:  "fnproject/fn-test-utils",
				Format: "http",
				ResourceConfig: models.ResourceConfig{
					Timeout:     testFn.Timeout,
					IdleTimeout: testFn.IdleTimeout,
					Memory:      testFn.Memory,
					CPUs:        testFn.CPUs,
				},
				// updated
				Config: map[string]string{
					"FIRST":  "1",
					"SECOND": "2",
					"THIRD":  "3",
				},
			}
			if !updated.Equals(expected) {
				t.Fatalf("Test UpdateRoute: expected updated `%v` but got `%v`", expected, updated)
			}

			// Update a config var, remove another. Add one Header, remove another.
			updated, err = ds.PutFn(ctx, &models.Fn{
				Name: testFn.Name,
				Config: map[string]string{
					"FIRST":  "first",
					"SECOND": "",
					"THIRD":  "3",
				},
			})
			if err != nil {
				t.Fatalf("Test UpdateRoute: unexpected error: %v", err)
			}
			expected = &models.Fn{
				// unchanged
				Name:   testFn.Name,
				Image:  "fnproject/fn-test-utils",
				Format: "http",
				ResourceConfig: models.ResourceConfig{
					Timeout:     testFn.Timeout,
					IdleTimeout: testFn.IdleTimeout,
					Memory:      testFn.Memory,
					CPUs:        testFn.CPUs,
				},
				// updated
				Config: map[string]string{
					"FIRST": "first",
					"THIRD": "3",
				},
			}
			if !updated.Equals(expected) {
				t.Fatalf("Test UpdateRoute: expected updated:\n`%v`\nbut got:\n`%v`", expected, updated)
			}
		}

		// Testing list fns
		fns, err := ds.GetFns(ctx, &models.FnFilter{PerPage: 1})
		if err != nil {
			t.Fatalf("Test GetFns: unexpected error %v", err)
		}
		if len(fns) == 0 {
			t.Fatal("Test GetFns: expected result count to be greater than 0")
		}
		if fns[0] == nil {
			t.Fatalf("Test GetFns: expected non-nil func")
		} else if fns[0].Name != testFn.Name {
			t.Fatalf("Test GetFns: expected `func.Name` to be `%s` but it was `%s`", testFn.Name, fns[0].Name)
		}

		// test pagination stuff
		r2 := *testFn
		r2.ID = id.New().String()
		r2.Name = "testa"
		r3 := *testFn
		r3.ID = id.New().String()
		r3.Name = "testb"

		if _, err = ds.PutFn(ctx, &r2); err != nil {
			t.Fatal(err)
		}
		if _, err = ds.PutFn(ctx, &r3); err != nil {
			t.Fatal(err)
		}

		fns, err = ds.GetFns(ctx, &models.FnFilter{PerPage: 1})
		if err != nil {
			t.Fatalf("Test GetFns: error: %s", err)
		}
		if len(fns) != 1 {
			t.Fatalf("Test GetFns: expected result count to be 1 but got %d", len(fns))
		} else if fns[0].Name != testFn.Name {
			t.Fatalf("Test GetFns: expected `func.Name` to be `%s` but it was `%s`", testFn.Name, fns[0].Name)
		}

		fns, err = ds.GetFns(ctx, &models.FnFilter{PerPage: 2, Cursor: fns[0].ID})
		if err != nil {
			t.Fatalf("Test GetFns: error: %s", err)
		}
		if len(fns) != 2 {
			t.Fatalf("Test GetFns: expected result count to be 2 but got %d", len(fns))
		} else if fns[0].Name != r2.Name {
			t.Fatalf("Test GetFns: expected `func.Name` to be `%s` but it was `%s`", r2.Name, fns[0].Name)
		} else if fns[1].Name != r3.Name {
			t.Fatalf("Test GetFns: expected `func.Name` to be `%s` but it was `%s`", r3.Name, fns[1].Name)
		}

		// TODO test weird ordering possibilities ?
		// TODO test prefix filtering

		// Testing func delete
		err = ds.RemoveFn(ctx, "")
		if err != models.ErrDatastoreEmptyFnName {
			t.Fatalf("Test RemoveFn(empty name): expected error `%v`, but it was `%v`", models.ErrDatastoreEmptyFnName, err)
		}

		err = ds.RemoveFn(ctx, testFn.Name)
		if err != nil {
			t.Fatalf("Test RemoveApp: unexpected error: %v", err)
		}

		fn, err := ds.GetFn(ctx, testFn.Name)
		if err != nil && err != models.ErrFnsNotFound {
			t.Fatalf("Test GetFn: expected error `%v`, but it was `%v`", models.ErrFnsNotFound, err)
		}
		if fn != nil {
			t.Fatalf("Test RemoveFn: failed to remove the func: %v", fn)
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

var testFn = &models.Fn{
	ID:     id.New().String(),
	Name:   "test",
	Image:  "fnproject/fn-test-utils",
	Format: "http",
	ResourceConfig: models.ResourceConfig{
		Timeout:     models.DefaultTimeout,
		IdleTimeout: models.DefaultIdleTimeout,
		Memory:      models.DefaultMemory,
	},
}
