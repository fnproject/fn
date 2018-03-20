package tests

import (
	"testing"

	"github.com/fnproject/fn_go/client/routes"
	"github.com/fnproject/fn_go/models"
)

func AssertRouteMatches(t *testing.T, expected *models.Route, got *models.Route) {

	if expected.Path != got.Path {
		t.Errorf("Route path mismatch. Expected: %v. Actual: %v", expected.Path, got.Path)
	}
	if expected.Image != got.Image {
		t.Errorf("Route image mismatch. Expected: %v. Actual: %v", expected.Image, got.Image)
	}
	if expected.Image != got.Image {
		t.Errorf("Route type mismatch. Expected: %v. Actual: %v", expected.Image, got.Image)
	}
	if expected.Format != got.Format {
		t.Errorf("Route format mismatch. Expected: %v. Actual: %v", expected.Format, got.Format)
	}

}

// PostRoute Creates a route and deletes the corresponding app (if created) on teardown
func (s *TestHarness) PostRoute(appName string, route *models.Route) (*routes.PostAppsAppRoutesOK, error) {
	cfg := &routes.PostAppsAppRoutesParams{
		App: appName,
		Body: &models.RouteWrapper{
			Route: route,
		},
		Context: s.Context,
	}
	ok, err := s.Client.Routes.PostAppsAppRoutes(cfg)

	if err == nil {
		s.createdApps[appName] = true
	}
	return ok, err

}

func (s *TestHarness) BasicRoute() *models.Route {
	return &models.Route{
		Format:      s.Format,
		Path:        s.RoutePath,
		Image:       s.Image,
		Type:        s.RouteType,
		Timeout:     &s.Timeout,
		IDLETimeout: &s.IdleTimeout,
	}
}

//GivenRouteExists creates a route using the specified arguments, failing the test if the creation fails, this tears down any apps that are created when the test is complete
func (s *TestHarness) GivenRouteExists(t *testing.T, appName string, route *models.Route) {
	_, err := s.PostRoute(appName, route)
	if err != nil {
		t.Fatalf("Expected route to be created, got %v", err)
	}

}

//RouteMustExist checks that a route exists, failing the test if it doesn't, returns the route
func (s *TestHarness) RouteMustExist(t *testing.T, appName string, routePath string) *models.Route {
	cfg := &routes.GetAppsAppRoutesRouteParams{
		App:     appName,
		Route:   routePath[1:],
		Context: s.Context,
	}

	routeResponse, err := s.Client.Routes.GetAppsAppRoutesRoute(cfg)
	if err != nil {
		t.Fatalf("Expected route %s %s to exist but got %v", appName, routePath, err)
	}
	return routeResponse.Payload.Route
}

//GivenRoutePatched applies a patch to a route, failing the test if this fails.
func (s *TestHarness) GivenRoutePatched(t *testing.T, appName, routeName string, rt *models.Route) {

	_, err := s.Client.Routes.PatchAppsAppRoutesRoute(&routes.PatchAppsAppRoutesRouteParams{
		App:     appName,
		Route:   routeName,
		Context: s.Context,
		Body: &models.RouteWrapper{
			Route: rt,
		},
	})

	if err != nil {
		t.Fatalf("Failed to patch route %s %s : %v", appName, routeName, err)
	}
}

func assertContainsRoute(routeModels []*models.Route, expectedRoute string) bool {
	for _, r := range routeModels {
		if r.Path == expectedRoute {
			return true
		}
	}
	return false
}

//PutRoute creates a route via PUT, tearing down any apps that are created when the test is complete
func (s *TestHarness) PutRoute(appName string, routePath string, route *models.Route) (*routes.PutAppsAppRoutesRouteOK, error) {
	cfg := &routes.PutAppsAppRoutesRouteParams{
		App:     appName,
		Context: s.Context,
		Route:   routePath,
		Body: &models.RouteWrapper{
			Route: route,
		},
	}

	resp, err := s.Client.Routes.PutAppsAppRoutesRoute(cfg)

	if err == nil {
		s.createdApps[appName] = true
	}

	return resp, err
}
