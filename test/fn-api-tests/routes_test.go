package tests

//
import (
	"testing"

	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn_go/models"
	"reflect"

	fnTest "github.com/fnproject/fn/test"
)

func TestRoutes(t *testing.T) {
	newRouteType := "sync"
	newRoutePath := id.New().String()
	t.Run("create-route-with-empty-type", func(t *testing.T) {
		t.Parallel()
		s := fnTest.SetupDefaultSuite()
		fnTest.CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		_, err := fnTest.CreateRouteNoAssert(s.Context, s.Client, s.AppName, s.RoutePath, s.Image, "", s.Format,
			s.RouteConfig, s.RouteHeaders)
		if err == nil {
			t.Errorf("Should fail with Invalid route Type.")
		}
		fnTest.DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("create-route", func(t *testing.T) {
		t.Parallel()
		s := fnTest.SetupDefaultSuite()
		fnTest.CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		fnTest.CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.Format, s.RouteConfig, s.RouteHeaders)
		fnTest.DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("list-and-find-route", func(t *testing.T) {
		t.Parallel()
		s := fnTest.SetupDefaultSuite()
		fnTest.CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		fnTest.CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.Format, s.RouteConfig, s.RouteHeaders)
		if !fnTest.AssertContainsRoute(fnTest.ListRoutes(t, s.Context, s.Client, s.AppName), s.RoutePath) {
			t.Errorf("Unable to find corresponding route `%v` in list", s.RoutePath)
		}
		fnTest.DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("can-get-corresponding-route", func(t *testing.T) {
		t.Parallel()
		s := fnTest.SetupDefaultSuite()
		fnTest.CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		fnTest.CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.Format, s.RouteConfig, s.RouteHeaders)

		rObjects := []*models.Route{fnTest.GetRoute(t, s.Context, s.Client, s.AppName, s.RoutePath)}
		if !fnTest.AssertContainsRoute(rObjects, s.RoutePath) {
			t.Errorf("Unable to find corresponding route `%v` in list", s.RoutePath)
		}

		fnTest.DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("can-update-route-info", func(t *testing.T) {
		t.Parallel()
		s := fnTest.SetupDefaultSuite()
		fnTest.CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		fnTest.CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.Format, s.RouteConfig, s.RouteHeaders)

		routeResp, err := fnTest.UpdateRoute(
			t, s.Context, s.Client,
			s.AppName, s.RoutePath,
			s.Image, newRouteType, s.Format,
			s.Memory, s.RouteConfig, s.RouteHeaders, "")

		fnTest.CheckRouteResponseError(t, err)
		fnTest.AssertRouteFields(t, routeResp.Payload.Route, s.RoutePath, s.Image, newRouteType, s.Format)

		fnTest.DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("patch-route-with-config", func(t *testing.T) {
		t.Parallel()
		s := fnTest.SetupDefaultSuite()
		fnTest.CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		fnTest.CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.Format, s.RouteConfig, s.RouteHeaders)

		newRouteConf := map[string]string{
			"A": "a",
		}

		routeResp, err := fnTest.UpdateRoute(
			t, s.Context, s.Client,
			s.AppName, s.RoutePath,
			s.Image, s.RouteType, s.Format,
			s.Memory, newRouteConf, s.RouteHeaders, "")

		fnTest.CheckRouteResponseError(t, err)
		fnTest.AssertRouteFields(t, routeResp.Payload.Route, s.RoutePath, s.Image, s.RouteType, s.Format)

		fnTest.DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("fail-to-update-route-path", func(t *testing.T) {
		t.Parallel()
		s := fnTest.SetupDefaultSuite()
		fnTest.CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		fnTest.CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.Format, s.RouteConfig, s.RouteHeaders)

		_, err := fnTest.UpdateRoute(
			t, s.Context, s.Client,
			s.AppName, s.RoutePath,
			s.Image, s.RouteType, s.Format,
			s.Memory, s.RouteConfig, s.RouteHeaders, newRoutePath)
		if err == nil {
			t.Errorf("Route path suppose to be immutable, but it's not.")
		}

		fnTest.DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("create-route-duplicate", func(t *testing.T) {
		t.Parallel()
		s := fnTest.SetupDefaultSuite()
		fnTest.CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		fnTest.CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.Format, s.RouteConfig, s.RouteHeaders)

		_, err := fnTest.CreateRouteNoAssert(s.Context, s.Client, s.AppName, s.Image, s.RoutePath,
			newRouteType, s.Format, s.RouteConfig, s.RouteHeaders)
		if err == nil {
			t.Errorf("Route duplicate error should appear, but it didn't")
		}

		fnTest.DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("can-delete-route", func(t *testing.T) {
		t.Parallel()
		s := fnTest.SetupDefaultSuite()
		fnTest.CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		fnTest.CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.Format, s.RouteConfig, s.RouteHeaders)

		fnTest.DeleteRoute(t, s.Context, s.Client, s.AppName, s.RoutePath)
		fnTest.DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("fail-to-delete-missing-route", func(t *testing.T) {
		t.Parallel()
		s := fnTest.SetupDefaultSuite()
		fnTest.CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})

		_, err := fnTest.DeleteRouteNoAssert(s.Context, s.Client, s.AppName, "dummy-route")
		if err == nil {
			t.Error("Delete from missing route must fail.")
		}
		fnTest.DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("deploy-route-without-existing-app", func(T *testing.T) {
		t.Parallel()
		s := fnTest.SetupDefaultSuite()
		fnTest.DeployRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType, s.Format, s.RouteConfig, s.RouteHeaders)
		fnTest.GetApp(t, s.Context, s.Client, s.AppName)
		fnTest.GetRoute(t, s.Context, s.Client, s.AppName, s.RoutePath)
		fnTest.DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("deploy-route-with-existing-app", func(T *testing.T) {
		s := fnTest.SetupDefaultSuite()
		fnTest.CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		fnTest.DeployRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType, s.Format, s.RouteConfig, s.RouteHeaders)
		fnTest.GetApp(t, s.Context, s.Client, s.AppName)
		fnTest.GetRoute(t, s.Context, s.Client, s.AppName, s.RoutePath)
		fnTest.DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("deploy-update-with-existing-route-and-app", func(T *testing.T) {
		newRouteType := "sync"
		s := fnTest.SetupDefaultSuite()
		fnTest.CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		fnTest.CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType,
			s.Format, s.RouteConfig, s.RouteHeaders)

		updatedRoute := fnTest.DeployRoute(
			t, s.Context, s.Client,
			s.AppName, s.RoutePath,
			s.Image, newRouteType,
			s.Format, s.RouteConfig, s.RouteHeaders)
		fnTest.AssertRouteFields(t, updatedRoute, s.RoutePath, s.Image, newRouteType, s.Format)

		fnTest.DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("multiple-deploy-route-with-headers", func(T *testing.T) {
		s := fnTest.SetupDefaultSuite()
		fnTest.CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		routeHeaders := map[string][]string{}
		routeHeaders["A"] = []string{"a"}
		routeHeaders["B"] = []string{"b"}
		fnTest.DeployRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType, s.Format, s.RouteConfig, routeHeaders)
		sameRoute := fnTest.DeployRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, s.RouteType, s.Format, s.RouteConfig, routeHeaders)
		if ok := reflect.DeepEqual(sameRoute.Headers, routeHeaders); !ok {
			t.Error("Route headers should remain the same after multiple deploys with exact the same parameters")
		}
		fnTest.DeleteApp(t, s.Context, s.Client, s.AppName)
	})
}
