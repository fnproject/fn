package tests

import (
	"context"
	"strings"
	"testing"
	"time"

	fn "github.com/funcy/functions_go/client"
	"github.com/funcy/functions_go/client/apps"
	"github.com/funcy/functions_go/client/routes"
	"github.com/funcy/functions_go/models"
	"gitlab-odx.oracle.com/odx/functions/fn/client"
)

type SuiteSetup struct {
	Context      context.Context
	Client       *fn.Functions
	AppName      string
	RoutePath    string
	Image        string
	RouteType    string
	Format       string
	Memory       int64
	RouteConfig  map[string]string
	RouteHeaders map[string][]string
}

func SetupDefaultSuite() *SuiteSetup {
	return &SuiteSetup{
		Context:      context.Background(),
		Client:       client.APIClient(),
		AppName:      "test-app",
		RoutePath:    "/hello",
		Image:        "funcy/hello",
		Format:       "default",
		RouteType:    "async",
		RouteConfig:  map[string]string{},
		RouteHeaders: map[string][]string{},
	}
}

func CheckAppResponseError(t *testing.T, err error) {
	if err != nil {
		switch err.(type) {

		case *apps.DeleteAppsAppDefault:
			msg := err.(*apps.DeleteAppsAppDefault).Payload.Error.Message
			code := err.(*apps.DeleteAppsAppDefault).Code()
			t.Fatalf("Unexpected error occurred: %v. Status code: %v", msg, code)
			return

		case *apps.PostAppsDefault:
			msg := err.(*apps.PostAppsDefault).Payload.Error.Message
			code := err.(*apps.PostAppsDefault).Code()
			t.Fatalf("Unexpected error occurred: %v. Status code: %v", msg, code)
			return

		case *apps.GetAppsAppNotFound:
			msg := err.(*apps.GetAppsAppNotFound).Payload.Error.Message
			if !strings.Contains("App not found", msg) {
				t.Fatalf("Unexpected error occurred: %v", msg)
				return
			}
			return

		case *apps.GetAppsAppDefault:
			msg := err.(*apps.GetAppsAppDefault).Payload.Error.Message
			code := err.(*apps.GetAppsAppDefault).Code()
			t.Fatalf("Unexpected error occurred: %v. Status code: %v", msg, code)
			return

		case *apps.PatchAppsAppDefault:
			msg := err.(*apps.PatchAppsAppDefault).Payload.Error.Message
			code := err.(*apps.PatchAppsAppDefault).Code()
			t.Fatalf("Unexpected error occurred: %v. Status code: %v", msg, code)
			return

		case *apps.PatchAppsAppNotFound:
			msg := err.(*apps.PatchAppsAppNotFound).Payload.Error.Message
			t.Fatalf("Unexpected error occurred: %v.", msg)
			return

		case *apps.PatchAppsAppBadRequest:
			msg := err.(*apps.PatchAppsAppBadRequest).Payload.Error.Message
			t.Fatalf("Unexpected error occurred: %v.", msg)
			return
		}
		t.Fatalf("Unable to determine type of error: %s", err)
	}

}

func CreateAppNoAssert(ctx context.Context, fnclient *fn.Functions, appName string, config map[string]string) (*apps.PostAppsOK, error) {
	cfg := &apps.PostAppsParams{
		Body: &models.AppWrapper{
			App: &models.App{
				Config: config,
				Name:   appName,
			},
		},
		Context: ctx,
	}
	cfg.WithTimeout(time.Second * 60)
	return fnclient.Apps.PostApps(cfg)
}

func CreateApp(t *testing.T, ctx context.Context, fnclient *fn.Functions, appName string, config map[string]string) {
	appPayload, err := CreateAppNoAssert(ctx, fnclient, appName, config)
	CheckAppResponseError(t, err)
	if !strings.Contains(appName, appPayload.Payload.App.Name) {
		t.Fatalf("App name mismatch.\nExpected: %v\nActual: %v",
			appName, appPayload.Payload.App.Name)
	}
}

func UpdateApp(t *testing.T, ctx context.Context, fnclient *fn.Functions, appName string, config map[string]string) *apps.PatchAppsAppOK {
	CreateApp(t, ctx, fnclient, appName, map[string]string{"A": "a"})
	cfg := &apps.PatchAppsAppParams{
		App: appName,
		Body: &models.AppWrapper{
			App: &models.App{
				Config: config,
				Name:   "",
			},
		},
		Context: ctx,
	}
	appPayload, err := fnclient.Apps.PatchAppsApp(cfg)
	CheckAppResponseError(t, err)
	return appPayload
}

func DeleteApp(t *testing.T, ctx context.Context, fnclient *fn.Functions, appName string) {
	cfg := &apps.DeleteAppsAppParams{
		App:     appName,
		Context: ctx,
	}
	cfg.WithTimeout(time.Second * 60)
	_, err := fnclient.Apps.DeleteAppsApp(cfg)
	CheckAppResponseError(t, err)
}

func CheckRouteResponseError(t *testing.T, err error) {
	if err != nil {
		switch err.(type) {

		case *routes.PostAppsAppRoutesDefault:
			msg := err.(*routes.PostAppsAppRoutesDefault).Payload.Error.Message
			code := err.(*routes.PostAppsAppRoutesDefault).Code()
			t.Fatalf("Unexpected error occurred: %v. Status code: %v", msg, code)
			return

		case *routes.PostAppsAppRoutesBadRequest:
			msg := err.(*routes.PostAppsAppRoutesBadRequest).Payload.Error.Message
			t.Fatalf("Unexpected error occurred: %v.", msg)
			return

		case *routes.PostAppsAppRoutesConflict:
			msg := err.(*routes.PostAppsAppRoutesConflict).Payload.Error.Message
			t.Fatalf("Unexpected error occurred: %v.", msg)
			return

		case *routes.GetAppsAppRoutesRouteNotFound:
			msg := err.(*routes.GetAppsAppRoutesRouteNotFound).Payload.Error.Message
			t.Fatalf("Unexpected error occurred: %v.", msg)
			return

		case *routes.GetAppsAppRoutesRouteDefault:
			msg := err.(*routes.GetAppsAppRoutesRouteDefault).Payload.Error.Message
			code := err.(*routes.GetAppsAppRoutesRouteDefault).Code()
			t.Fatalf("Unexpected error occurred: %v. Status code: %v", msg, code)
			return

		case *routes.DeleteAppsAppRoutesRouteNotFound:
			msg := err.(*routes.DeleteAppsAppRoutesRouteNotFound).Payload.Error.Message
			t.Fatalf("Unexpected error occurred: %v.", msg)
			return

		case *routes.DeleteAppsAppRoutesRouteDefault:
			msg := err.(*routes.DeleteAppsAppRoutesRouteDefault).Payload.Error.Message
			code := err.(*routes.DeleteAppsAppRoutesRouteDefault).Code()
			t.Fatalf("Unexpected error occurred: %v. Status code: %v", msg, code)
			return
		case *routes.GetAppsAppRoutesNotFound:
			msg := err.(*routes.GetAppsAppRoutesNotFound).Payload.Error.Message
			t.Fatalf("Unexpected error occurred: %v.", msg)
			return

		case *routes.GetAppsAppRoutesDefault:
			msg := err.(*routes.GetAppsAppRoutesDefault).Payload.Error.Message
			code := err.(*routes.GetAppsAppRoutesDefault).Code()
			t.Fatalf("Unexpected error occurred: %v. Status code: %v", msg, code)
			return

		case *routes.PatchAppsAppRoutesRouteBadRequest:
			msg := err.(*routes.PatchAppsAppRoutesRouteBadRequest).Payload.Error.Message
			t.Fatalf("Unexpected error occurred: %v.", msg)
			return

		case *routes.PatchAppsAppRoutesRouteNotFound:
			msg := err.(*routes.PatchAppsAppRoutesRouteNotFound).Payload.Error.Message
			t.Fatalf("Unexpected error occurred: %v.", msg)
			return

		case *routes.PatchAppsAppRoutesRouteDefault:
			msg := err.(*routes.PatchAppsAppRoutesRouteDefault).Payload.Error.Message
			code := err.(*routes.PatchAppsAppRoutesRouteDefault).Code()
			t.Fatalf("Unexpected error occurred: %v. Status code: %v", msg, code)
			return

		}
		t.Fatalf("Unable to determine type of error: %s", err)
	}
}

func logRoute(t *testing.T, routeObject *models.Route) {
	t.Logf("Route path: %v", routeObject.Path)
	t.Logf("Route image: %v", routeObject.Image)
	t.Logf("Route type: %v", routeObject.Type)
	t.Logf("Route timeout: %vs", *routeObject.Timeout)
	t.Logf("Route idle timeout: %vs", *routeObject.IDLETimeout)
}

func assertRouteFields(t *testing.T, routeObject *models.Route, path, image, routeType string) {

	logRoute(t, routeObject)
	rPath := routeObject.Path
	rImage := routeObject.Image
	rType := routeObject.Type
	rTimeout := *routeObject.Timeout
	rIdleTimeout := *routeObject.IDLETimeout
	if rPath != path {
		t.Fatalf("Route path mismatch. Expected: %v. Actual: %v", path, rPath)
	}
	if rImage != image {
		t.Fatalf("Route image mismatch. Expected: %v. Actual: %v", image, rImage)
	}
	if rType != routeType {
		t.Fatalf("Route type mismatch. Expected: %v. Actual: %v", routeType, rType)
	}
	if rTimeout == 0 {
		t.Fatal("Route timeout should have default value of 30 seconds, but got 0 seconds")
	}
	if rIdleTimeout == 0 {
		t.Fatal("Route idle timeout should have default value of 30 seconds, but got 0 seconds")
	}

}

func createRoute(ctx context.Context, fnclient *fn.Functions, appName, image, routePath, routeType string, routeConfig map[string]string, headers map[string][]string) (*routes.PostAppsAppRoutesOK, error) {
	cfg := &routes.PostAppsAppRoutesParams{
		App: appName,
		Body: &models.RouteWrapper{
			Route: &models.Route{
				Config:  routeConfig,
				Headers: headers,
				Image:   image,
				Path:    routePath,
				Type:    routeType,
			},
		},
		Context: ctx,
	}
	cfg.WithTimeout(time.Second * 60)
	return fnclient.Routes.PostAppsAppRoutes(cfg)

}

func CreateRoute(t *testing.T, ctx context.Context, fnclient *fn.Functions, appName, routePath, image, routeType string, routeConfig map[string]string, headers map[string][]string) {
	routeResponse, err := createRoute(ctx, fnclient, appName, image, routePath, routeType, routeConfig, headers)
	CheckRouteResponseError(t, err)

	assertRouteFields(t, routeResponse.Payload.Route, routePath, image, routeType)
}

func deleteRoute(ctx context.Context, fnclient *fn.Functions, appName, routePath string) (*routes.DeleteAppsAppRoutesRouteOK, error) {
	cfg := &routes.DeleteAppsAppRoutesRouteParams{
		App:     appName,
		Route:   routePath,
		Context: ctx,
	}
	cfg.WithTimeout(time.Second * 60)
	return fnclient.Routes.DeleteAppsAppRoutesRoute(cfg)
}

func DeleteRoute(t *testing.T, ctx context.Context, fnclient *fn.Functions, appName, routePath string) {
	_, err := deleteRoute(ctx, fnclient, appName, routePath)
	CheckRouteResponseError(t, err)
}

func ListRoutes(t *testing.T, ctx context.Context, fnclient *fn.Functions, appName string) []*models.Route {
	cfg := &routes.GetAppsAppRoutesParams{
		App:     appName,
		Context: ctx,
	}
	cfg.WithTimeout(time.Second * 60)
	routesResponse, err := fnclient.Routes.GetAppsAppRoutes(cfg)
	CheckRouteResponseError(t, err)
	return routesResponse.Payload.Routes
}

func GetRoute(t *testing.T, ctx context.Context, fnclient *fn.Functions, appName, routePath string) *models.Route {
	cfg := &routes.GetAppsAppRoutesRouteParams{
		App:     appName,
		Route:   routePath,
		Context: ctx,
	}
	cfg.WithTimeout(time.Second * 60)
	routeResponse, err := fnclient.Routes.GetAppsAppRoutesRoute(cfg)
	CheckRouteResponseError(t, err)
	return routeResponse.Payload.Route
}

func UpdateRoute(t *testing.T, ctx context.Context, fnclient *fn.Functions, appName, routePath, image, routeType, format string, memory int64, routeConfig map[string]string, headers map[string][]string, newRoutePath string) (*routes.PatchAppsAppRoutesRouteOK, error) {

	routeObject := GetRoute(t, ctx, fnclient, appName, routePath)
	if routeObject.Config == nil {
		routeObject.Config = map[string]string{}
	}

	if routeObject.Headers == nil {
		routeObject.Headers = map[string][]string{}
	}
	logRoute(t, routeObject)

	routeObject.Path = ""
	if newRoutePath != "" {
		routeObject.Path = newRoutePath
	}

	if routeConfig != nil {
		for k, v := range routeConfig {
			if string(k[0]) == "-" {
				delete(routeObject.Config, string(k[1:]))
				continue
			}
			routeObject.Config[k] = v
		}
	}
	if headers != nil {
		for k, v := range headers {
			if string(k[0]) == "-" {
				delete(routeObject.Headers, k)
				continue
			}
			routeObject.Headers[k] = v
		}
	}
	if image != "" {
		routeObject.Image = image
	}
	if format != "" {
		routeObject.Format = format
	}
	if routeType != "" {
		routeObject.Type = routeType
	}
	if memory > 0 {
		routeObject.Memory = memory
	}

	cfg := &routes.PatchAppsAppRoutesRouteParams{
		App:     appName,
		Context: ctx,
		Body: &models.RouteWrapper{
			Route: routeObject,
		},
		Route: routePath,
	}
	cfg.WithTimeout(time.Second * 60)

	t.Log("Calling update")

	return fnclient.Routes.PatchAppsAppRoutesRoute(cfg)
}

func assertContainsRoute(routeModels []*models.Route, expectedRoute string) bool {
	for _, r := range routeModels {
		if r.Path == expectedRoute {
			return true
		}
	}
	return false
}
