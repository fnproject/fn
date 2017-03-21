package datastoretest

import (
	"bytes"
	"context"
	"log"
	"testing"

	"github.com/iron-io/functions/api/models"

	"net/http"
	"net/url"
	"os"
	"reflect"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
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

func GetContainerHostIP() string {
	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost == "" {
		return "127.0.0.1"
	}
	parts, _ := url.Parse(dockerHost)
	return parts.Hostname()
}

func Test(t *testing.T, ds models.Datastore) {
	buf := setLogBuffer()

	ctx := context.Background()

	t.Run("apps", func(t *testing.T) {
		// Testing insert app
		_, err := ds.InsertApp(ctx, nil)
		if err != models.ErrDatastoreEmptyApp {
			t.Log(buf.String())
			t.Fatalf("Test InsertApp(nil): expected error `%v`, but it was `%v`", models.ErrDatastoreEmptyApp, err)
		}

		_, err = ds.InsertApp(ctx, &models.App{})
		if err != models.ErrDatastoreEmptyAppName {
			t.Log(buf.String())
			t.Fatalf("Test InsertApp(&{}): expected error `%v`, but it was `%v`", models.ErrDatastoreEmptyAppName, err)
		}

		inserted, err := ds.InsertApp(ctx, testApp)
		if err != nil {
			t.Log(buf.String())
			t.Fatalf("Test InsertApp: error when storing new app: %s", err)
		}
		if !reflect.DeepEqual(*inserted, *testApp) {
			t.Log(buf.String())
			t.Fatalf("Test InsertApp: expected to insert:\n%v\nbut got:\n%v", testApp, inserted)
		}

		_, err = ds.InsertApp(ctx, testApp)
		if err != models.ErrAppsAlreadyExists {
			t.Log(buf.String())
			t.Fatalf("Test InsertApp duplicated: expected error `%v`, but it was `%v`", models.ErrAppsAlreadyExists, err)
		}

		{
			// Set a config var
			updated, err := ds.UpdateApp(ctx,
				&models.App{Name: testApp.Name, Config: map[string]string{"TEST": "1"}})
			if err != nil {
				t.Log(buf.String())
				t.Fatalf("Test UpdateApp: error when updating app: %v", err)
			}
			expected := &models.App{Name: testApp.Name, Config: map[string]string{"TEST": "1"}}
			if !reflect.DeepEqual(*updated, *expected) {
				t.Log(buf.String())
				t.Fatalf("Test UpdateApp: expected updated `%v` but got `%v`", expected, updated)
			}

			// Set a different var (without clearing the existing)
			updated, err = ds.UpdateApp(ctx,
				&models.App{Name: testApp.Name, Config: map[string]string{"OTHER": "TEST"}})
			if err != nil {
				t.Log(buf.String())
				t.Fatalf("Test UpdateApp: error when updating app: %v", err)
			}
			expected = &models.App{Name: testApp.Name, Config: map[string]string{"TEST": "1", "OTHER": "TEST"}}
			if !reflect.DeepEqual(*updated, *expected) {
				t.Log(buf.String())
				t.Fatalf("Test UpdateApp: expected updated `%v` but got `%v`", expected, updated)
			}

			// Delete a var
			updated, err = ds.UpdateApp(ctx,
				&models.App{Name: testApp.Name, Config: map[string]string{"TEST": ""}})
			if err != nil {
				t.Log(buf.String())
				t.Fatalf("Test UpdateApp: error when updating app: %v", err)
			}
			expected = &models.App{Name: testApp.Name, Config: map[string]string{"OTHER": "TEST"}}
			if !reflect.DeepEqual(*updated, *expected) {
				t.Log(buf.String())
				t.Fatalf("Test UpdateApp: expected updated `%v` but got `%v`", expected, updated)
			}
		}

		// Testing get app
		_, err = ds.GetApp(ctx, "")
		if err != models.ErrDatastoreEmptyAppName {
			t.Log(buf.String())
			t.Fatalf("Test GetApp: expected error to be %v, but it was %s", models.ErrDatastoreEmptyAppName, err)
		}

		app, err := ds.GetApp(ctx, testApp.Name)
		if err != nil {
			t.Log(buf.String())
			t.Fatalf("Test GetApp: error: %s", err)
		}
		if app.Name != testApp.Name {
			t.Log(buf.String())
			t.Fatalf("Test GetApp: expected `app.Name` to be `%s` but it was `%s`", app.Name, testApp.Name)
		}

		// Testing list apps
		apps, err := ds.GetApps(ctx, &models.AppFilter{})
		if err != nil {
			t.Log(buf.String())
			t.Fatalf("Test GetApps: unexpected error %v", err)
		}
		if len(apps) == 0 {
			t.Fatal("Test GetApps: expected result count to be greater than 0")
		}
		if apps[0].Name != testApp.Name {
			t.Log(buf.String())
			t.Fatalf("Test GetApps: expected `app.Name` to be `%s` but it was `%s`", app.Name, testApp.Name)
		}

		apps, err = ds.GetApps(ctx, &models.AppFilter{Name: "Tes%"})
		if err != nil {
			t.Log(buf.String())
			t.Fatalf("Test GetApps(filter): unexpected error %v", err)
		}
		if len(apps) == 0 {
			t.Fatal("Test GetApps(filter): expected result count to be greater than 0")
		}

		// Testing app delete
		err = ds.RemoveApp(ctx, "")
		if err != models.ErrDatastoreEmptyAppName {
			t.Log(buf.String())
			t.Fatalf("Test RemoveApp: expected error `%v`, but it was `%v`", models.ErrDatastoreEmptyAppName, err)
		}

		err = ds.RemoveApp(ctx, testApp.Name)
		if err != nil {
			t.Log(buf.String())
			t.Fatalf("Test RemoveApp: error: %s", err)
		}
		app, err = ds.GetApp(ctx, testApp.Name)
		if err != models.ErrAppsNotFound {
			t.Log(buf.String())
			t.Fatalf("Test GetApp(removed): expected error `%v`, but it was `%v`", models.ErrAppsNotFound, err)
		}
		if app != nil {
			t.Log(buf.String())
			t.Fatal("Test RemoveApp: failed to remove the app")
		}

		// Test update inexistent app
		_, err = ds.UpdateApp(ctx, &models.App{
			Name: testApp.Name,
			Config: map[string]string{
				"TEST": "1",
			},
		})
		if err != models.ErrAppsNotFound {
			t.Log(buf.String())
			t.Fatalf("Test UpdateApp(inexistent): expected error `%v`, but it was `%v`", models.ErrAppsNotFound, err)
		}
	})

	t.Run("routes", func(t *testing.T) {
		// Insert app again to test routes
		_, err := ds.InsertApp(ctx, testApp)
		if err != nil && err != models.ErrAppsAlreadyExists {
			t.Log(buf.String())
			t.Fatalf("Test InsertRoute Prep: failed to insert app: ", err)
		}

		// Testing insert route
		{
			_, err = ds.InsertRoute(ctx, nil)
			if err != models.ErrDatastoreEmptyRoute {
				t.Log(buf.String())
				t.Fatalf("Test InsertRoute(nil): expected error `%v`, but it was `%v`", models.ErrDatastoreEmptyRoute, err)
			}

			_, err = ds.InsertRoute(ctx, &models.Route{AppName: "notreal", Path: "/test"})
			if err != models.ErrAppsNotFound {
				t.Log(buf.String())
				t.Fatalf("Test InsertRoute: expected error `%v`, but it was `%v`", models.ErrAppsNotFound, err)
			}

			_, err = ds.InsertRoute(ctx, testRoute)
			if err != nil {
				t.Log(buf.String())
				t.Fatalf("Test InsertRoute: error when storing new route: %s", err)
			}

			_, err = ds.InsertRoute(ctx, testRoute)
			if err != models.ErrRoutesAlreadyExists {
				t.Log(buf.String())
				t.Fatalf("Test InsertRoute duplicated: expected error to be `%v`, but it was `%v`", models.ErrRoutesAlreadyExists, err)
			}
		}

		// Testing get
		{
			_, err = ds.GetRoute(ctx, "a", "")
			if err != models.ErrDatastoreEmptyRoutePath {
				t.Log(buf.String())
				t.Fatalf("Test GetRoute(empty route path): expected error `%v`, but it was `%v`", models.ErrDatastoreEmptyRoutePath, err)
			}

			_, err = ds.GetRoute(ctx, "", "a")
			if err != models.ErrDatastoreEmptyAppName {
				t.Log(buf.String())
				t.Fatalf("Test GetRoute(empty app name): expected error `%v`, but it was `%v`", models.ErrDatastoreEmptyAppName, err)
			}

			route, err := ds.GetRoute(ctx, testApp.Name, testRoute.Path)
			if err != nil {
				t.Log(buf.String())
				t.Fatalf("Test GetRoute: unexpected error %v", err)
			}
			var expected models.Route = *testRoute
			if !reflect.DeepEqual(*route, expected) {
				t.Log(buf.String())
				t.Fatalf("Test InsertApp: expected to insert:\n%v\nbut got:\n%v", expected, route)
			}
		}

		// Testing update
		{
			// Update some fields, and add 3 configs and 3 headers.
			updated, err := ds.UpdateRoute(ctx, &models.Route{
				AppName: testRoute.AppName,
				Path:    testRoute.Path,
				Timeout: 100,
				Config: map[string]string{
					"FIRST":  "1",
					"SECOND": "2",
					"THIRD":  "3",
				},
				Headers: http.Header{
					"First":  []string{"test"},
					"Second": []string{"test", "test"},
					"Third":  []string{"test", "test2"},
				},
			})
			if err != nil {
				t.Log(buf.String())
				t.Fatalf("Test UpdateRoute: unexpected error: %v", err)
			}
			expected := &models.Route{
				// unchanged
				AppName: testRoute.AppName,
				Path:    testRoute.Path,
				Image:   "iron/hello",
				Type:    "sync",
				Format:  "http",
				// updated
				Timeout: 100,
				Config: map[string]string{
					"FIRST":  "1",
					"SECOND": "2",
					"THIRD":  "3",
				},
				Headers: http.Header{
					"First":  []string{"test"},
					"Second": []string{"test", "test"},
					"Third":  []string{"test", "test2"},
				},
			}
			if !reflect.DeepEqual(*updated, *expected) {
				t.Log(buf.String())
				t.Fatalf("Test UpdateRoute: expected updated `%v` but got `%v`", expected, updated)
			}

			// Update a config var, remove another. Add one Header, remove another.
			updated, err = ds.UpdateRoute(ctx, &models.Route{
				AppName: testRoute.AppName,
				Path:    testRoute.Path,
				Config: map[string]string{
					"FIRST":  "first",
					"SECOND": "",
					"THIRD":  "3",
				},
				Headers: http.Header{
					"First":  []string{"test2"},
					"Second": nil,
				},
			})
			if err != nil {
				t.Log(buf.String())
				t.Fatalf("Test UpdateRoute: unexpected error: %v", err)
			}
			expected = &models.Route{
				// unchanged
				AppName: testRoute.AppName,
				Path:    testRoute.Path,
				Image:   "iron/hello",
				Type:    "sync",
				Format:  "http",
				Timeout: 100,
				// updated
				Config: map[string]string{
					"FIRST": "first",
					"THIRD": "3",
				},
				Headers: http.Header{
					"First": []string{"test", "test2"},
					"Third": []string{"test", "test2"},
				},
			}
			if !reflect.DeepEqual(*updated, *expected) {
				t.Log(buf.String())
				t.Fatalf("Test UpdateRoute: expected updated:\n`%v`\nbut got:\n`%v`", expected, updated)
			}
		}

		// Testing list routes
		routes, err := ds.GetRoutesByApp(ctx, testApp.Name, &models.RouteFilter{})
		if err != nil {
			t.Log(buf.String())
			t.Fatalf("Test GetRoutesByApp: unexpected error %v", err)
		}
		if len(routes) == 0 {
			t.Fatal("Test GetRoutesByApp: expected result count to be greater than 0")
		}
		if routes[0] == nil {
			t.Log(buf.String())
			t.Fatalf("Test GetRoutes: expected non-nil route")
		} else if routes[0].Path != testRoute.Path {
			t.Log(buf.String())
			t.Fatalf("Test GetRoutes: expected `app.Name` to be `%s` but it was `%s`", testRoute.Path, routes[0].Path)
		}

		routes, err = ds.GetRoutesByApp(ctx, testApp.Name, &models.RouteFilter{Image: testRoute.Image})
		if err != nil {
			t.Log(buf.String())
			t.Fatalf("Test GetRoutesByApp: unexpected error %v", err)
		}
		if len(routes) == 0 {
			t.Fatal("Test GetRoutesByApp: expected result count to be greater than 0")
		}
		if routes[0] == nil {
			t.Log(buf.String())
			t.Fatalf("Test GetRoutes: expected non-nil route")
		} else if routes[0].Path != testRoute.Path {
			t.Log(buf.String())
			t.Fatalf("Test GetRoutes: expected `app.Name` to be `%s` but it was `%s`", testRoute.Path, routes[0].Path)
		}

		routes, err = ds.GetRoutesByApp(ctx, "notreal", nil)
		if err != nil {
			t.Log(buf.String())
			t.Fatalf("Test GetRoutesByApp: error: %s", err)
		}
		if len(routes) != 0 {
			t.Fatalf("Test GetRoutesByApp: expected result count to be 0 but got %d", len(routes))
		}

		// Testing list routes
		routes, err = ds.GetRoutes(ctx, &models.RouteFilter{Image: testRoute.Image})
		if err != nil {
			t.Log(buf.String())
			t.Fatalf("Test GetRoutes: error: %s", err)
		}
		if len(routes) == 0 {
			t.Fatal("Test GetRoutes: expected result count to be greater than 0")
		}
		if routes[0].Path != testRoute.Path {
			t.Log(buf.String())
			t.Fatalf("Test GetRoutes: expected `app.Name` to be `%s` but it was `%s`", testRoute.Path, routes[0].Path)
		}

		// Testing route delete
		err = ds.RemoveRoute(ctx, "", "")
		if err != models.ErrDatastoreEmptyAppName {
			t.Log(buf.String())
			t.Fatalf("Test RemoveRoute(empty app name): expected error `%v`, but it was `%v`", models.ErrDatastoreEmptyAppName, err)
		}

		err = ds.RemoveRoute(ctx, "a", "")
		if err != models.ErrDatastoreEmptyRoutePath {
			t.Log(buf.String())
			t.Fatalf("Test RemoveRoute(empty route path): expected error `%v`, but it was `%v`", models.ErrDatastoreEmptyRoutePath, err)
		}

		err = ds.RemoveRoute(ctx, testRoute.AppName, testRoute.Path)
		if err != nil {
			t.Log(buf.String())
			t.Fatalf("Test RemoveApp: unexpected error: %v", err)
		}

		route, err := ds.GetRoute(ctx, testRoute.AppName, testRoute.Path)
		if err != nil && err != models.ErrRoutesNotFound {
			t.Log(buf.String())
			t.Fatalf("Test GetRoute: expected error `%v`, but it was `%v`", models.ErrRoutesNotFound, err)
		}
		if route != nil {
			t.Log(buf.String())
			t.Fatalf("Test RemoveApp: failed to remove the route: %v", route)
		}

		_, err = ds.UpdateRoute(ctx, &models.Route{
			AppName: testRoute.AppName,
			Path:    testRoute.Path,
			Image:   "test",
		})
		if err != models.ErrRoutesNotFound {
			t.Log(buf.String())
			t.Fatalf("Test UpdateRoute inexistent: expected error to be `%v`, but it was `%v`", models.ErrRoutesNotFound, err)
		}
	})

	t.Run("put-get", func(t *testing.T) {
		// Testing Put/Get
		err := ds.Put(ctx, nil, nil)
		if err != models.ErrDatastoreEmptyKey {
			t.Log(buf.String())
			t.Fatalf("Test Put(nil,nil): expected error `%v`, but it was `%v`", models.ErrDatastoreEmptyKey, err)
		}

		err = ds.Put(ctx, []byte("test"), []byte("success"))
		if err != nil {
			t.Log(buf.String())
			t.Fatalf("Test Put: unexpected error: %v", err)
		}

		val, err := ds.Get(ctx, []byte("test"))
		if err != nil {
			t.Log(buf.String())
			t.Fatalf("Test Put: unexpected error: %v", err)
		}
		if string(val) != "success" {
			t.Log(buf.String())
			t.Fatalf("Test Get: expected value to be `%v`, but it was `%v`", "success", string(val))
		}

		err = ds.Put(ctx, []byte("test"), nil)
		if err != nil {
			t.Log(buf.String())
			t.Fatalf("Test Put: unexpected error: %v", err)
		}

		val, err = ds.Get(ctx, []byte("test"))
		if err != nil {
			t.Log(buf.String())
			t.Fatalf("Test Put: unexpected error: %v", err)
		}
		if string(val) != "" {
			t.Log(buf.String())
			t.Fatalf("Test Get: expected value to be `%v`, but it was `%v`", "", string(val))
		}
	})
}

var testApp = &models.App{
	Name: "Test",
}

var testRoute = &models.Route{
	AppName: testApp.Name,
	Path:    "/test",
	Image:   "iron/hello",
	Type:    "sync",
	Format:  "http",
}
