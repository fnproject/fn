package tests

//
import (
	"testing"

	"github.com/fnproject/fn/api/id"
	"github.com/funcy/functions_go/models"
)

func TestRoutes(t *testing.T) {
	newRouteType := "sync"
	newRoutePath := id.New().String()

	t.Run("create-route", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.RouteConfig, s.RouteHeaders)
		DeleteRoute(t, s.Context, s.Client, s.AppName, s.RoutePath)
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("list-and-find-route", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.RouteConfig, s.RouteHeaders)
		if !assertContainsRoute(ListRoutes(t, s.Context, s.Client, s.AppName), s.RoutePath) {
			t.Errorf("Unable to find corresponding route `%v` in list", s.RoutePath)
		}
		DeleteRoute(t, s.Context, s.Client, s.AppName, s.RoutePath)
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("can-get-corresponding-route", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.RouteConfig, s.RouteHeaders)

		rObjects := []*models.Route{GetRoute(t, s.Context, s.Client, s.AppName, s.RoutePath)}
		if !assertContainsRoute(rObjects, s.RoutePath) {
			t.Errorf("Unable to find corresponding route `%v` in list", s.RoutePath)
		}

		DeleteRoute(t, s.Context, s.Client, s.AppName, s.RoutePath)
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("can-update-route-info", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.RouteConfig, s.RouteHeaders)

		routeResp, err := UpdateRoute(
			t, s.Context, s.Client,
			s.AppName, s.RoutePath,
			s.Image, newRouteType, s.Format,
			s.Memory, s.RouteConfig, s.RouteHeaders, "")

		CheckRouteResponseError(t, err)
		assertRouteFields(t, routeResp.Payload.Route, s.RoutePath, s.Image, newRouteType)

		DeleteRoute(t, s.Context, s.Client, s.AppName, s.RoutePath)
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("fail-to-update-route-path", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.RouteConfig, s.RouteHeaders)

		_, err := UpdateRoute(
			t, s.Context, s.Client,
			s.AppName, s.RoutePath,
			s.Image, s.RouteType, s.Format,
			s.Memory, s.RouteConfig, s.RouteHeaders, newRoutePath)
		if err == nil {
			t.Errorf("Route path suppose to be immutable, but it's not.")
		}

		DeleteRoute(t, s.Context, s.Client, s.AppName, s.RoutePath)
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("create-route-duplicate", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.RouteConfig, s.RouteHeaders)

		_, err := createRoute(s.Context, s.Client, s.AppName, s.Image, s.RoutePath, newRouteType, s.RouteConfig, s.RouteHeaders)
		if err == nil {
			t.Errorf("Route duplicate error should appear, but it didn't")
		}

		DeleteRoute(t, s.Context, s.Client, s.AppName, s.RoutePath)
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("can-delete-route", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.RouteConfig, s.RouteHeaders)

		DeleteRoute(t, s.Context, s.Client, s.AppName, s.RoutePath)
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("fail-to-delete-missing-route", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})

		_, err := deleteRoute(s.Context, s.Client, s.AppName, "dummy-route")
		if err == nil {
			t.Error("Delete from missing route must fail.")
		}
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})
}
