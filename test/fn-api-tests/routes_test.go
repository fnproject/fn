package tests

import (
	"testing"

	"reflect"

	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn_go/client/apps"
	"github.com/fnproject/fn_go/client/routes"
	"github.com/fnproject/fn_go/models"
)

func TestShouldRejectEmptyRouteType(t *testing.T) {
	t.Parallel()
	s := SetupHarness()
	defer s.Cleanup()

	s.GivenAppExists(t, &models.App{Name: s.AppName})

	_, err := s.PostRoute(s.AppName, &models.Route{
		Path:   s.RoutePath,
		Image:  s.Image,
		Type:   "v",
		Format: s.Format,
	})

	if err == nil {
		t.Errorf("Should fail with Invalid route Type.")
	}
}

func TestCanCreateRoute(t *testing.T) {
	t.Parallel()
	s := SetupHarness()
	defer s.Cleanup()

	s.GivenAppExists(t, &models.App{Name: s.AppName})
	_, err := s.PostRoute(s.AppName, &models.Route{
		Path:   s.RoutePath,
		Image:  s.Image,
		Format: s.Format,
	})

	if err != nil {
		t.Errorf("expected route success, got %v", err)
	}
	// TODO validate route returned matches request
}

func TestListRoutes(t *testing.T) {
	t.Parallel()
	s := SetupHarness()
	defer s.Cleanup()

	s.GivenAppExists(t, &models.App{Name: s.AppName})
	s.GivenRouteExists(t, s.AppName, s.BasicRoute())

	cfg := &routes.GetAppsAppRoutesParams{
		App:     s.AppName,
		Context: s.Context,
	}

	routesResponse, err := s.Client.Routes.GetAppsAppRoutes(cfg)

	if err != nil {
		t.Fatalf("Expecting list routes to be successful, got %v", err)
	}
	if !assertContainsRoute(routesResponse.Payload.Routes, s.RoutePath) {
		t.Errorf("Unable to find corresponding route `%v` in list", s.RoutePath)
	}
}

func TestInspectRoute(t *testing.T) {
	t.Parallel()
	s := SetupHarness()
	defer s.Cleanup()

	s.GivenAppExists(t, &models.App{Name: s.AppName})
	newRt := s.BasicRoute()
	s.GivenRouteExists(t, s.AppName, newRt)

	resp, err := s.Client.Routes.GetAppsAppRoutesRoute(&routes.GetAppsAppRoutesRouteParams{
		App:     s.AppName,
		Route:   newRt.Path[1:],
		Context: s.Context,
	})

	if err != nil {
		t.Fatalf("Failed to get route %s, %v", s.RoutePath, err)
	}

	gotRt := resp.Payload.Route

	AssertRouteMatches(t, newRt, gotRt)

}

var validRouteUpdates = []struct {
	name    string
	update  *models.Route
	extract func(*models.Route) interface{}
}{
	{"route type (sync)", &models.Route{Type: "sync"}, func(m *models.Route) interface{} { return m.Type }},
	{"route type (async)", &models.Route{Type: "async"}, func(m *models.Route) interface{} { return m.Type }},
	{"format (json)", &models.Route{Format: "json"}, func(m *models.Route) interface{} { return m.Format }},
	{"format (default)", &models.Route{Format: "default"}, func(m *models.Route) interface{} { return m.Format }},
	// ...
}

func TestCanUpdateRouteAttributes(t *testing.T) {
	t.Parallel()

	for _, tci := range validRouteUpdates {
		tc := tci
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := SetupHarness()
			defer s.Cleanup()

			s.GivenAppExists(t, &models.App{Name: s.AppName})
			s.GivenRouteExists(t, s.AppName, s.BasicRoute())

			routeResp, err := s.Client.Routes.PatchAppsAppRoutesRoute(
				&routes.PatchAppsAppRoutesRouteParams{
					App:     s.AppName,
					Context: s.Context,
					Route:   s.RoutePath,
					Body: &models.RouteWrapper{
						Route: tc.update,
					},
				},
			)
			if err != nil {
				t.Fatalf("Failed to patch route, got %v", err)
			}

			got := tc.extract(routeResp.Payload.Route)
			change := tc.extract(tc.update)
			if !reflect.DeepEqual(got, change) {
				t.Errorf("Expected value in response tobe %v but was %v", change, got)
			}
		})
	}

}

func TestCanUpdateRouteConfig(t *testing.T) {
	t.Parallel()
	for _, tci := range updateConfigCases {
		tc := tci
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := SetupHarness()
			defer s.Cleanup()
			s.GivenAppExists(t, &models.App{Name: s.AppName})
			route := s.BasicRoute()
			route.Config = tc.intialConfig

			s.GivenRouteExists(t, s.AppName, route)

			routeResp, err := s.Client.Routes.PatchAppsAppRoutesRoute(
				&routes.PatchAppsAppRoutesRouteParams{
					App:   s.AppName,
					Route: s.RoutePath,
					Body: &models.RouteWrapper{
						Route: &models.Route{
							Config: tc.change,
						},
					},
					Context: s.Context,
				},
			)

			if err != nil {
				t.Fatalf("Failed to patch route, got %v", err)
			}
			actual := routeResp.Payload.Route.Config
			if !ConfigEquivalent(actual, tc.expected) {
				t.Errorf("Expected config : %v after update, got %v", tc.expected, actual)
			}

		})
	}

}

func TestSetRouteAnnotationsOnCreate(t *testing.T) {
	t.Parallel()
	for _, tci := range createAnnotationsValidCases {
		// iterator mutation meets parallelism... pfft
		tc := tci
		t.Run("valid_"+tc.name, func(t *testing.T) {
			t.Parallel()
			s := SetupHarness()
			defer s.Cleanup()

			s.GivenAppExists(t, &models.App{
				Name: s.AppName,
			})
			rt := s.BasicRoute()
			rt.Annotations = tc.annotations

			route, err := s.Client.Routes.PostAppsAppRoutes(&routes.PostAppsAppRoutesParams{
				App:     s.AppName,
				Context: s.Context,
				Body: &models.RouteWrapper{
					Route: rt,
				},
			})

			if err != nil {
				t.Fatalf("Failed to create route with valid annotations %v got error %v", tc.annotations, err)
			}

			gotMd := route.Payload.Route.Annotations
			if !AnnotationsEquivalent(gotMd, tc.annotations) {
				t.Errorf("Returned annotations %v does not match set annotations %v", gotMd, tc.annotations)
			}

			getRoute := s.RouteMustExist(t, s.AppName, s.RoutePath)

			if !AnnotationsEquivalent(getRoute.Annotations, tc.annotations) {
				t.Errorf("GET annotations '%v' does not match set annotations %v", getRoute.Annotations, tc.annotations)
			}

		})
	}

	for _, tci := range createAnnotationsErrorCases {
		// iterator mutation meets parallelism... pfft
		tc := tci
		t.Run("invalid_"+tc.name, func(ti *testing.T) {
			ti.Parallel()
			s := SetupHarness()
			defer s.Cleanup()

			_, err := s.PostApp(&models.App{
				Name:        s.AppName,
				Annotations: tc.annotations,
			})

			if err == nil {
				t.Fatalf("Created app with invalid annotations %v but expected error", tc.annotations)
			}

			if _, ok := err.(*apps.PostAppsBadRequest); !ok {
				t.Errorf("Expecting bad request for invalid annotations, got %v", err)
			}

		})
	}
}

func TestSetRouteMetadataOnPatch(t *testing.T) {
	t.Parallel()

	for _, tci := range updateAnnotationsValidCases {
		// iterator mutation meets parallelism... pfft
		tc := tci
		t.Run("valid_"+tc.name, func(t *testing.T) {
			t.Parallel()
			s := SetupHarness()
			defer s.Cleanup()

			s.GivenAppExists(t, &models.App{Name: s.AppName})
			rt := s.BasicRoute()
			rt.Annotations = tc.initial
			s.GivenRouteExists(t, s.AppName, rt)

			res, err := s.Client.Routes.PatchAppsAppRoutesRoute(&routes.PatchAppsAppRoutesRouteParams{
				App:     s.AppName,
				Route:   s.RoutePath[1:],
				Context: s.Context,
				Body: &models.RouteWrapper{
					Route: &models.Route{
						Annotations: tc.change,
					},
				},
			})

			if err != nil {
				t.Fatalf("Failed to patch annotations with %v on route: %v", tc.change, err)
			}

			gotMd := res.Payload.Route.Annotations
			if !AnnotationsEquivalent(gotMd, tc.expected) {
				t.Errorf("Returned annotations %v does not match set annotations %v", gotMd, tc.expected)
			}

			getRoute := s.RouteMustExist(t, s.AppName, s.RoutePath)

			if !AnnotationsEquivalent(getRoute.Annotations, tc.expected) {
				t.Errorf("GET annotations '%v' does not match set annotations %v", getRoute.Annotations, tc.expected)
			}
		})
	}

	for _, tci := range updateAnnotationsErrorCases {
		// iterator mutation meets parallelism... pfft
		tc := tci
		t.Run("invalid_"+tc.name, func(t *testing.T) {
			t.Parallel()
			s := SetupHarness()
			defer s.Cleanup()

			s.GivenAppExists(t, &models.App{
				Name: s.AppName,
			})
			rt := s.BasicRoute()
			rt.Annotations = tc.initial
			s.GivenRouteExists(t, s.AppName, rt)

			_, err := s.Client.Routes.PatchAppsAppRoutesRoute(&routes.PatchAppsAppRoutesRouteParams{
				App:     s.AppName,
				Route:   s.RoutePath[1:],
				Context: s.Context,
				Body: &models.RouteWrapper{
					Route: &models.Route{
						Annotations: tc.change,
					},
				},
			})

			if err == nil {
				t.Errorf("patched route with invalid annotations %v but expected error", tc.change)
			}
			if _, ok := err.(*routes.PatchAppsAppRoutesRouteBadRequest); !ok {
				t.Errorf("Expecting bad request for invalid annotations, got %v", err)
			}

		})
	}
}

func TestCantUpdateRoutePath(t *testing.T) {

	t.Parallel()
	s := SetupHarness()
	defer s.Cleanup()

	s.GivenAppExists(t, &models.App{Name: s.AppName})
	s.GivenRouteExists(t, s.AppName, s.BasicRoute())

	_, err := s.Client.Routes.PatchAppsAppRoutesRoute(
		&routes.PatchAppsAppRoutesRouteParams{
			App:   s.AppName,
			Route: s.RoutePath,
			Body: &models.RouteWrapper{
				Route: &models.Route{
					Path: id.New().String(),
				},
			},
		})
	if err == nil {
		t.Fatalf("Expected error when patching route")
	}
	if _, ok := err.(*routes.PatchAppsAppRoutesRouteBadRequest); ok {
		t.Errorf("Error should be bad request when updating route path  ")
	}

}

func TestRoutePreventsDuplicate(t *testing.T) {
	t.Parallel()
	s := SetupHarness()
	defer s.Cleanup()

	s.GivenAppExists(t, &models.App{Name: s.AppName})
	s.GivenRouteExists(t, s.AppName, s.BasicRoute())

	_, err := s.PostRoute(s.AppName, s.BasicRoute())

	if err == nil {
		t.Errorf("Route duplicate error should appear, but it didn't")
	}
	if _, ok := err.(*routes.PostAppsAppRoutesConflict); !ok {
		t.Errorf("Error should be a conflict when creating a new route, got %v", err)
	}
}

func TestCanDeleteRoute(t *testing.T) {
	t.Parallel()
	s := SetupHarness()
	defer s.Cleanup()

	s.GivenAppExists(t, &models.App{Name: s.AppName})
	s.GivenRouteExists(t, s.AppName, s.BasicRoute())

	_, err := s.Client.Routes.DeleteAppsAppRoutesRoute(&routes.DeleteAppsAppRoutesRouteParams{
		App:     s.AppName,
		Route:   s.RoutePath,
		Context: s.Context,
	})

	if err != nil {
		t.Errorf("Expected success when deleting existing route, got %v", err)
	}
}

func TestCantDeleteMissingRoute(t *testing.T) {
	t.Parallel()
	s := SetupHarness()
	defer s.Cleanup()

	s.GivenAppExists(t, &models.App{Name: s.AppName})

	_, err := s.Client.Routes.DeleteAppsAppRoutesRoute(&routes.DeleteAppsAppRoutesRouteParams{
		App:     s.AppName,
		Route:   s.RoutePath,
		Context: s.Context,
	})

	if err == nil {
		t.Fatalf("Expected error when deleting non-existing route, got none")
	}

	if _, ok := err.(*routes.DeleteAppsAppRoutesRouteNotFound); !ok {
		t.Fatalf("Expected not-found when deleting non-existing route, got %v", err)

	}
}

func TestPutRouteCreatesNewApp(t *testing.T) {
	t.Parallel()
	s := SetupHarness()
	defer s.Cleanup()

	_, err := s.PutRoute(s.AppName, s.RoutePath, s.BasicRoute())

	if err != nil {
		t.Fatalf("Expected new route to be created, got %v", err)
	}

	s.AppMustExist(t, s.AppName)
	s.RouteMustExist(t, s.AppName, s.RoutePath)

}

func TestPutRouteToExistingApp(t *testing.T) {
	s := SetupHarness()
	defer s.Cleanup()

	s.GivenAppExists(t, &models.App{Name: s.AppName})
	_, err := s.PutRoute(s.AppName, s.RoutePath, s.BasicRoute())
	if err != nil {
		t.Fatalf("Failed to create route, got error %v", err)
	}
	s.AppMustExist(t, s.AppName)
	s.RouteMustExist(t, s.AppName, s.RoutePath)
}

func TestPutRouteUpdatesRoute(t *testing.T) {
	newRouteType := "sync"
	s := SetupHarness()
	defer s.Cleanup()

	s.GivenAppExists(t, &models.App{Name: s.AppName})
	s.GivenRouteExists(t, s.AppName, s.BasicRoute())

	changed := s.BasicRoute()
	changed.Type = newRouteType

	updatedRoute, err := s.PutRoute(s.AppName, s.RoutePath, changed)

	if err != nil {
		t.Fatalf("Failed to update route, got %v", err)
	}
	got := updatedRoute.Payload.Route.Type
	if got != newRouteType {
		t.Errorf("expected type to be %v after update, got %v", newRouteType, got)
	}
}

func TestPutIsIdempotentForHeaders(t *testing.T) {
	s := SetupHarness()
	defer s.Cleanup()

	s.GivenAppExists(t, &models.App{Name: s.AppName})

	routeHeaders := map[string][]string{}
	routeHeaders["A"] = []string{"a"}
	routeHeaders["B"] = []string{"b"}

	r1 := s.BasicRoute()
	r1.Headers = routeHeaders

	updatedRoute1, err := s.PutRoute(s.AppName, s.RoutePath, r1)

	if err != nil {
		t.Fatalf("Failed to update route, got %v", err)
	}
	if firstMatches := reflect.DeepEqual(routeHeaders, updatedRoute1.Payload.Route.Headers); !firstMatches {
		t.Errorf("Route headers should remain the same after multiple deploys with exact the same parameters '%v' != '%v'", routeHeaders, updatedRoute1.Payload.Route.Headers)
	}

	updatedRoute2, err := s.PutRoute(s.AppName, s.RoutePath, r1)

	if bothmatch := reflect.DeepEqual(updatedRoute1.Payload.Route.Headers, updatedRoute2.Payload.Route.Headers); !bothmatch {
		t.Error("Route headers should remain the same after multiple deploys with exact the same parameters")
	}
}
