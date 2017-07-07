package tests

import (
	"testing"

	"github.com/funcy/functions_go/models"
)

func TestRoutes(t *testing.T) {
	s := SetupDefaultSuite()

	newRouteType := "sync"
	newRoutePath := "/new-hello"

	CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})

	t.Run("create-route", func(t *testing.T) {
		CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.RouteConfig, s.RouteHeaders)
		t.Logf("Test `%v` passed.", t.Name())
	})

	t.Run("list-and-find-route", func(t *testing.T) {
		if !assertContainsRoute(ListRoutes(t, s.Context, s.Client, s.AppName), s.RoutePath) {
			t.Fatalf("Unable to find corresponding route `%v` in list", s.RoutePath)
		}
		t.Logf("Test `%v` passed.", t.Name())
	})

	t.Run("can-get-corresponding-route", func(t *testing.T) {
		rObjects := []*models.Route{GetRoute(t, s.Context, s.Client, s.AppName, s.RoutePath)}
		if !assertContainsRoute(rObjects, s.RoutePath) {
			t.Fatalf("Unable to find corresponding route `%v` in list", s.RoutePath)
		}
		t.Logf("Test `%v` passed.", t.Name())
	})

	t.Run("can-update-route-info", func(t *testing.T) {
		routeResp, err := UpdateRoute(
			t, s.Context, s.Client,
			s.AppName, s.RoutePath,
			s.Image, newRouteType, s.Format,
			s.Memory, s.RouteConfig, s.RouteHeaders, "")

		CheckRouteResponseError(t, err)
		assertRouteFields(t, routeResp.Payload.Route, s.RoutePath, s.Image, newRouteType)

		t.Logf("Test `%v` passed.", t.Name())
	})

	t.Run("fail-to-update-route-path", func(t *testing.T) {
		_, err := UpdateRoute(
			t, s.Context, s.Client,
			s.AppName, s.RoutePath,
			s.Image, s.RouteType, s.Format,
			s.Memory, s.RouteConfig, s.RouteHeaders, newRoutePath)
		if err == nil {
			t.Fatalf("Route path suppose to be immutable, but it's not.")
		}
		t.Logf("Test `%v` passed.", t.Name())
	})

	t.Run("create-route-duplicate", func(t *testing.T) {
		_, err := createRoute(s.Context, s.Client, s.AppName, s.Image, s.RoutePath, newRouteType, s.RouteConfig, s.RouteHeaders)
		if err == nil {
			t.Fatalf("Route duplicate error should appear, but it didn't")
		}
	})

	t.Run("can-delete-route", func(t *testing.T) {
		DeleteRoute(t, s.Context, s.Client, s.AppName, s.RoutePath)
		t.Logf("Test `%v` passed.", t.Name())
	})

	t.Run("fail-to-delete-missing-route", func(t *testing.T) {
		_, err := deleteRoute(s.Context, s.Client, s.AppName, "dummy-route")
		if err == nil {
			t.Fatal("Delete from missing route must fail.")
		}
	})

	DeleteApp(t, s.Context, s.Client, s.AppName)
}
