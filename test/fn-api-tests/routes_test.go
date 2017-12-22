package tests

import (
	"testing"

	"github.com/fnproject/fn/api/id"
	api_models "github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn_go/models"
)

func TestRoutes(t *testing.T) {
	newRouteType := "sync"
	newRoutePath := id.New().String()
	t.Run("create-route-with-empty-type", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		_, err := createRoute(s.Context, s.Client, s.AppName, s.RoutePath, s.Image, "",
			s.Format, s.Timeout, s.IdleTimeout, s.RouteConfig, s.RouteHeaders)
		if err == nil {
			t.Errorf("Should fail with Invalid route Type.")
		}
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("create-route", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.Format, s.Timeout, s.IdleTimeout, s.RouteConfig, s.RouteHeaders)
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("list-and-find-route", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.Format, s.Timeout, s.IdleTimeout, s.RouteConfig, s.RouteHeaders)
		if !assertContainsRoute(ListRoutes(t, s.Context, s.Client, s.AppName), s.RoutePath) {
			t.Errorf("Unable to find corresponding route `%v` in list", s.RoutePath)
		}
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("can-get-corresponding-route", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.Format, s.Timeout, s.IdleTimeout, s.RouteConfig, s.RouteHeaders)

		rObjects := []*models.Route{GetRoute(t, s.Context, s.Client, s.AppName, s.RoutePath)}
		if !assertContainsRoute(rObjects, s.RoutePath) {
			t.Errorf("Unable to find corresponding route `%v` in list", s.RoutePath)
		}

		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("can-update-route-info", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.Format, s.Timeout, s.IdleTimeout, s.RouteConfig, s.RouteHeaders)

		routeResp, err := UpdateRoute(
			t, s.Context, s.Client,
			s.AppName, s.RoutePath,
			s.Image, newRouteType, s.Format,
			s.Memory, s.RouteConfig, s.RouteHeaders, "")

		CheckRouteResponseError(t, err)
		assertRouteFields(t, routeResp.Payload.Route, s.RoutePath, s.Image, newRouteType, s.Format)

		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("patch-route-with-config", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.Format, s.Timeout, s.IdleTimeout, s.RouteConfig, s.RouteHeaders)

		newRouteConf := map[string]string{
			"A": "a",
		}

		routeResp, err := UpdateRoute(
			t, s.Context, s.Client,
			s.AppName, s.RoutePath,
			s.Image, s.RouteType, s.Format,
			s.Memory, newRouteConf, s.RouteHeaders, "")

		CheckRouteResponseError(t, err)
		assertRouteFields(t, routeResp.Payload.Route, s.RoutePath, s.Image, s.RouteType, s.Format)

		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("fail-to-update-route-path", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.Format, s.Timeout, s.IdleTimeout, s.RouteConfig, s.RouteHeaders)

		_, err := UpdateRoute(
			t, s.Context, s.Client,
			s.AppName, s.RoutePath,
			s.Image, s.RouteType, s.Format,
			s.Memory, s.RouteConfig, s.RouteHeaders, newRoutePath)
		if err == nil {
			t.Errorf("Route path suppose to be immutable, but it's not.")
		}

		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("create-route-duplicate", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.Format, s.Timeout, s.IdleTimeout, s.RouteConfig, s.RouteHeaders)

		_, err := createRoute(s.Context, s.Client, s.AppName, s.Image, s.RoutePath,
			newRouteType, s.Format, s.Timeout, s.IdleTimeout, s.RouteConfig, s.RouteHeaders)
		if err == nil {
			t.Errorf("Route duplicate error should appear, but it didn't")
		}

		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("can-delete-route", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.Format, s.Timeout, s.IdleTimeout, s.RouteConfig, s.RouteHeaders)

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

	t.Run("deploy-route-without-existing-app", func(T *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		DeployRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType, s.Format, s.RouteConfig, s.RouteHeaders)
		GetApp(t, s.Context, s.Client, s.AppName)
		GetRoute(t, s.Context, s.Client, s.AppName, s.RoutePath)
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("deploy-route-with-existing-app", func(T *testing.T) {
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		DeployRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType, s.Format, s.RouteConfig, s.RouteHeaders)
		GetApp(t, s.Context, s.Client, s.AppName)
		GetRoute(t, s.Context, s.Client, s.AppName, s.RoutePath)
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("deploy-update-with-existing-route-and-app", func(T *testing.T) {
		newRouteType := "sync"
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.Format, s.Timeout, s.IdleTimeout, s.RouteConfig, s.RouteHeaders)

		updatedRoute := DeployRoute(
			t, s.Context, s.Client,
			s.AppName, s.RoutePath,
			s.Image, newRouteType,
			s.Format, s.RouteConfig, s.RouteHeaders)
		assertRouteFields(t, updatedRoute, s.RoutePath, s.Image, newRouteType, s.Format)

		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("multiple-deploy-route-with-headers", func(T *testing.T) {
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		routeHeaders := map[string][]string{}
		routeHeaders["A"] = []string{"a"}
		routeHeaders["B"] = []string{"b"}
		DeployRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType, s.Format, s.RouteConfig, routeHeaders)
		sameRoute := DeployRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType, s.Format, s.RouteConfig, routeHeaders)
		if !api_models.Headers(sameRoute.Headers).Equals(api_models.Headers(routeHeaders)) {
			t.Error("Route headers should remain the same after multiple deploys with exact the same parameters")
		}
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

}
