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
	//if rTimeout == 0 {
	//	t.Error("Route timeout should have default value of 30 seconds, but got 0 seconds")
	//}
	//if rIdleTimeout == 0 {
	//	t.Error("Route idle timeout should have default value of 30 seconds, but got 0 seconds")
	//}
	if expected.Format != got.Format {
		t.Errorf("Route format mismatch. Expected: %v. Actual: %v", expected.Format, got.Format)
	}

}

func (s *SuiteSetup) PostRoute(appName string, route *models.Route) (*routes.PostAppsAppRoutesOK, error) {
	cfg := &routes.PostAppsAppRoutesParams{
		App: appName,
		Body: &models.RouteWrapper{
			Route: route,
		},
		Context: s.Context,
	}
	ok, err := s.Client.Routes.PostAppsAppRoutes(cfg)

	if err == nil {
		s.createdRoutes = append(s.createdRoutes, &appRoute{appName: appName, routeName: ok.Payload.Route.Path})
	}
	return ok, err

}

func (s *SuiteSetup) BasicRoute() *models.Route {
	return &models.Route{
		Format:      s.Format,
		Path:        s.RoutePath,
		Image:       s.Image,
		Type:        s.RouteType,
		Timeout:     &s.Timeout,
		IDLETimeout: &s.IdleTimeout,
	}
}

func (s *SuiteSetup) GivenRouteExists(t *testing.T, appName string, route *models.Route) {
	_, err := s.PostRoute(appName, route)
	if err != nil {
		t.Fatalf("Expected route to be created, got %v", err)
	}

}

func (s *SuiteSetup) RequireRouteExists(t *testing.T, appName string, routePath string) *models.Route {
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

func (s *SuiteSetup) GivenRoutePatched(t *testing.T, appName, routeName string, rt *models.Route) {

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

func (s *SuiteSetup) PutRoute(appName string, routePath string, route *models.Route) (*routes.PutAppsAppRoutesRouteOK, error) {
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
		s.createdApps = append(s.createdApps, appName)
	}
	return resp, err
}
