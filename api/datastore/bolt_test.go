package datastore

import (
	"os"
	"testing"

	"github.com/iron-io/functions/api/models"
)

var tmpBolt = "/tmp/func_test_bolt.db"

func prepareBolt(t *testing.T) (models.Datastore, func()) {
	ds, err := New("bolt://" + tmpBolt)
	if err != nil {
		t.Fatal("Error when creating datastore: %s", err)
	}
	return ds, func() {
		os.Remove(tmpBolt)
	}
}

func TestBolt(t *testing.T) {
	ds, close := prepareBolt(t)
	defer close()

	testApp := &models.App{
		Name: "Test",
	}

	testRoute := &models.Route{
		AppName: testApp.Name,
		Path:    "/test",
		Image:   "iron/hello",
	}

	// Testing store app
	_, err := ds.StoreApp(nil)
	if err == nil {
		t.Fatalf("Test StoreApp: expected error when using nil app", err)
	}

	_, err = ds.StoreApp(testApp)
	if err != nil {
		t.Fatalf("Test StoreApp: error when Bolt was storing new app: %s", err)
	}

	// Testing get app
	_, err = ds.GetApp("")
	if err == nil {
		t.Fatalf("Test GetApp: expected error when using empty app name", err)
	}

	app, err := ds.GetApp(testApp.Name)
	if err != nil {
		t.Fatalf("Test GetApp: error: %s", err)
	}
	if app.Name != testApp.Name {
		t.Fatalf("Test GetApp: expected `app.Name` to be `%s` but it was `%s`", app.Name, testApp.Name)
	}

	// Testing list apps
	apps, err := ds.GetApps(&models.AppFilter{})
	if err != nil {
		t.Fatalf("Test GetApps: error: %s", err)
	}
	if len(apps) == 0 {
		t.Fatal("Test GetApps: expected result count to be greater than 0")
	}
	if apps[0].Name != testApp.Name {
		t.Fatalf("Test GetApps: expected `app.Name` to be `%s` but it was `%s`", app.Name, testApp.Name)
	}

	// Testing app delete
	err = ds.RemoveApp("")
	if err == nil {
		t.Fatalf("Test RemoveApp: expected error when using empty app name", err)
	}

	err = ds.RemoveApp(testApp.Name)
	if err != nil {
		t.Fatalf("Test RemoveApp: error: %s", err)
	}
	app, err = ds.GetApp(testApp.Name)
	if err != nil {
		t.Fatalf("Test GetApp: error: %s", err)
	}
	if app != nil {
		t.Fatalf("Test RemoveApp: failed to remove the app")
	}

	// Store app again to test routes
	ds.StoreApp(testApp)

	// Testing store route
	_, err = ds.StoreRoute(nil)
	if err == nil {
		t.Fatalf("Test StoreRoute: expected error when using nil route", err)
	}

	_, err = ds.StoreRoute(testRoute)
	if err != nil {
		t.Fatalf("Test StoreReoute: error when Bolt was storing new route: %s", err)
	}

	// Testing get
	_, err = ds.GetRoute("a", "")
	if err == nil {
		t.Fatalf("Test GetRoute: expected error when using empty route name", err)
	}

	_, err = ds.GetRoute("", "a")
	if err == nil {
		t.Fatalf("Test GetRoute: expected error when using empty app name", err)
	}

	route, err := ds.GetRoute(testApp.Name, testRoute.Path)
	if err != nil {
		t.Fatalf("Test GetRoute: error: %s", err)
	}
	if route.Path != testRoute.Path {
		t.Fatalf("Test GetRoute: expected `route.Name` to be `%s` but it was `%s`", route.Path, testRoute.Path)
	}

	// Testing list routes
	routes, err := ds.GetRoutesByApp(testApp.Name, &models.RouteFilter{})
	if err != nil {
		t.Fatalf("Test GetRoutes: error: %s", err)
	}
	if len(routes) == 0 {
		t.Fatal("Test GetRoutes: expected result count to be greater than 0")
	}
	if routes[0].Path != testRoute.Path {
		t.Fatalf("Test GetRoutes: expected `app.Name` to be `%s` but it was `%s`", testRoute.Path, routes[0].Path)
	}

	// Testing list routes
	routes, err = ds.GetRoutes(&models.RouteFilter{Image: testRoute.Image})
	if err != nil {
		t.Fatalf("Test GetRoutes: error: %s", err)
	}
	if len(routes) == 0 {
		t.Fatal("Test GetRoutes: expected result count to be greater than 0")
	}
	if routes[0].Path != testRoute.Path {
		t.Fatalf("Test GetRoutes: expected `app.Name` to be `%s` but it was `%s`", testRoute.Path, routes[0].Path)
	}

	// Testing app delete
	err = ds.RemoveRoute("", "")
	if err == nil {
		t.Fatalf("Test RemoveRoute: expected error when using empty app name", err)
	}

	err = ds.RemoveRoute("a", "")
	if err == nil {
		t.Fatalf("Test RemoveRoute: expected error when using empty route name", err)
	}

	err = ds.RemoveRoute(testRoute.AppName, testRoute.Path)
	if err != nil {
		t.Fatalf("Test RemoveApp: error: %s", err)
	}

	route, err = ds.GetRoute(testRoute.AppName, testRoute.Path)
	if err != nil {
		t.Fatalf("Test GetRoute: error: %s", err)
	}
	if route != nil {
		t.Fatalf("Test RemoveApp: failed to remove the route")
	}
}
