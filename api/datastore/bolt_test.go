package datastore

import (
	"context"
	"os"
	"testing"

	"github.com/iron-io/functions/api/models"
)

const tmpBolt = "/tmp/func_test_bolt.db"

func TestBolt(t *testing.T) {
	buf := setLogBuffer()

	ctx := context.Background()

	os.Remove(tmpBolt)
	ds, err := New("bolt://" + tmpBolt)
	if err != nil {
		t.Fatalf("Error when creating datastore: %v", err)
	}

	// Testing insert app
	_, err = ds.InsertApp(ctx, nil)
	if err != models.ErrDatastoreEmptyApp {
		t.Log(buf.String())
		t.Fatalf("Test InsertApp(nil): expected error `%v`, but it was `%v`", models.ErrDatastoreEmptyApp, err)
	}

	_, err = ds.InsertApp(ctx, &models.App{})
	if err != models.ErrDatastoreEmptyAppName {
		t.Log(buf.String())
		t.Fatalf("Test InsertApp(nil): expected error `%v`, but it was `%v`", models.ErrDatastoreEmptyAppName, err)
	}

	_, err = ds.InsertApp(ctx, testApp)
	if err != nil {
		t.Log(buf.String())
		t.Fatalf("Test InsertApp: error when Bolt was storing new app: %s", err)
	}

	_, err = ds.InsertApp(ctx, testApp)
	if err != models.ErrAppsAlreadyExists {
		t.Log(buf.String())
		t.Fatalf("Test InsertApp duplicated: expected error `%v`, but it was `%v`", models.ErrAppsAlreadyExists, err)
	}

	_, err = ds.UpdateApp(ctx, &models.App{
		Name: testApp.Name,
		Config: map[string]string{
			"TEST": "1",
		},
	})
	if err != nil {
		t.Log(buf.String())
		t.Fatalf("Test UpdateApp: error when Bolt was updating app: %v", err)
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
		t.Fatalf("Test GetApp: unexpected error: %s", err)
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
		t.Fatalf("Test GetApp: expected error to be `%v`, but it was `%v`", models.ErrAppsNotFound, err)
	}
	if app != nil {
		t.Log(buf.String())
		t.Fatalf("Test RemoveApp: failed to remove the app")
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
		t.Fatalf("Test UpdateApp inexistent: expected error to be %v, but it was %v", models.ErrAppsNotFound, err)
	}

	// Insert app again to test routes
	ds.InsertApp(ctx, testApp)

	// Testing insert route
	_, err = ds.InsertRoute(ctx, nil)
	if err == models.ErrDatastoreEmptyRoute {
		t.Log(buf.String())
		t.Fatalf("Test InsertRoute(nil): expected error `%v`, but it was `%v`", models.ErrDatastoreEmptyRoute, err)
	}

	_, err = ds.InsertRoute(ctx, testRoute)
	if err != nil {
		t.Log(buf.String())
		t.Fatalf("Test InsertRoute: error when Bolt was storing new route: %s", err)
	}

	_, err = ds.InsertRoute(ctx, testRoute)
	if err != models.ErrRoutesAlreadyExists {
		t.Log(buf.String())
		t.Fatalf("Test InsertRoute duplicated: expected error to be `%v`, but it was `%v`", models.ErrRoutesAlreadyExists, err)
	}

	_, err = ds.UpdateRoute(ctx, testRoute)
	if err != nil {
		t.Log(buf.String())
		t.Fatalf("Test UpdateRoute: unexpected error: %v", err)
	}

	// Testing get
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
	if route.Path != testRoute.Path {
		t.Log(buf.String())
		t.Fatalf("Test GetRoute: expected `route.Path` to be `%s` but it was `%s`", route.Path, testRoute.Path)
	}

	// Testing list routes
	routes, err := ds.GetRoutesByApp(ctx, testApp.Name, &models.RouteFilter{})
	if err != nil {
		t.Log(buf.String())
		t.Fatalf("Test GetRoutes: unexpected error %v", err)
	}
	if len(routes) == 0 {
		t.Fatal("Test GetRoutes: expected result count to be greater than 0")
	}
	if routes[0].Path != testRoute.Path {
		t.Log(buf.String())
		t.Fatalf("Test GetRoutes: expected `app.Name` to be `%s` but it was `%s`", testRoute.Path, routes[0].Path)
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

	apps, err = ds.GetApps(ctx, &models.AppFilter{Name: "Tes%"})
	if err != nil {
		t.Log(buf.String())
		t.Fatalf("Test GetApps(filter): unexpected error %v", err)
	}
	if len(apps) == 0 {
		t.Fatal("Test GetApps(filter): expected result count to be greater than 0")
	}

	// Testing app delete
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

	_, err = ds.UpdateRoute(ctx, &models.Route{
		AppName: testRoute.AppName,
		Path:    testRoute.Path,
		Image:   "test",
	})
	if err != models.ErrRoutesNotFound {
		t.Log(buf.String())
		t.Fatalf("Test UpdateRoute inexistent: expected error to be `%v`, but it was `%v`", models.ErrRoutesNotFound, err)
	}

	route, err = ds.GetRoute(ctx, testRoute.AppName, testRoute.Path)
	if err != models.ErrRoutesNotFound {
		t.Log(buf.String())
		t.Fatalf("Test RemoveApp: failed to remove the route")
	}

	// Testing Put/Get
	err = ds.Put(ctx, nil, nil)
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
}
