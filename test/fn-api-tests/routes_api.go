package tests

import (
	"context"
	"testing"

	"github.com/fnproject/fn_go/client"
	"github.com/fnproject/fn_go/client/routes"
	"github.com/fnproject/fn_go/models"
)

func CheckRouteResponseError(t *testing.T, e error) {
	if e != nil {
		switch err := e.(type) {
		case *routes.PostAppsAppRoutesDefault:
			t.Errorf("Unexpected error occurred: %v. Status code: %v", err.Payload.Error.Message, err.Code())
			t.FailNow()
		case *routes.PostAppsAppRoutesBadRequest:
			t.Errorf("Unexpected error occurred: %v.", err.Payload.Error.Message)
			t.FailNow()
		case *routes.PostAppsAppRoutesConflict:
			t.Errorf("Unexpected error occurred: %v.", err.Payload.Error.Message)
			t.FailNow()
		case *routes.GetAppsAppRoutesRouteNotFound:
			t.Errorf("Unexpected error occurred: %v.", err.Payload.Error.Message)
			t.FailNow()
		case *routes.GetAppsAppRoutesRouteDefault:
			t.Errorf("Unexpected error occurred: %v. Status code: %v", err.Payload.Error.Message, err.Code())
			t.FailNow()
		case *routes.DeleteAppsAppRoutesRouteNotFound:
			t.Errorf("Unexpected error occurred: %v.", err.Payload.Error.Message)
			t.FailNow()
		case *routes.DeleteAppsAppRoutesRouteDefault:
			t.Errorf("Unexpected error occurred: %v. Status code: %v", err.Payload.Error.Message, err.Code())
			t.FailNow()
		case *routes.GetAppsAppRoutesNotFound:
			t.Errorf("Unexpected error occurred: %v.", err.Payload.Error.Message)
			t.FailNow()
		case *routes.GetAppsAppRoutesDefault:
			t.Errorf("Unexpected error occurred: %v. Status code: %v", err.Payload.Error.Message, err.Code())
			t.FailNow()
		case *routes.PatchAppsAppRoutesRouteBadRequest:
			t.Errorf("Unexpected error occurred: %v.", err.Payload.Error.Message)
			t.FailNow()
		case *routes.PatchAppsAppRoutesRouteNotFound:
			t.Errorf("Unexpected error occurred: %v.", err.Payload.Error.Message)
			t.FailNow()
		case *routes.PatchAppsAppRoutesRouteDefault:
			t.Errorf("Unexpected error occurred: %v. Status code: %v", err.Payload.Error.Message, err.Code())
		case *routes.PutAppsAppRoutesRouteBadRequest:
			t.Errorf("Unexpected error occurred: %v.", err.Payload.Error.Message)
		case *routes.PutAppsAppRoutesRouteDefault:
			t.Errorf("Unexpected error occurred: %v. Status code: %v", err.Payload.Error.Message, err.Code())
			t.FailNow()
		default:
			t.Errorf("Unable to determine type of error: %s", err)
			t.FailNow()
		}
	}
}

func assertRouteFields(t *testing.T, routeObject *models.Route, path, image, routeType, routeFormat string) {

	rPath := routeObject.Path
	rImage := routeObject.Image
	rType := routeObject.Type
	rTimeout := *routeObject.Timeout
	rIdleTimeout := *routeObject.IDLETimeout
	rFormat := routeObject.Format

	if rPath != path {
		t.Errorf("Route path mismatch. Expected: %v. Actual: %v", path, rPath)
	}
	if rImage != image {
		t.Errorf("Route image mismatch. Expected: %v. Actual: %v", image, rImage)
	}
	if rType != routeType {
		t.Errorf("Route type mismatch. Expected: %v. Actual: %v", routeType, rType)
	}
	if rTimeout == 0 {
		t.Error("Route timeout should have default value of 30 seconds, but got 0 seconds")
	}
	if rIdleTimeout == 0 {
		t.Error("Route idle timeout should have default value of 30 seconds, but got 0 seconds")
	}
	if rFormat != routeFormat {
		t.Errorf("Route format mismatch. Expected: %v. Actual: %v", routeFormat, rFormat)
	}

}

func createRoute(ctx context.Context, fnclient *client.Fn, appName, image, routePath, routeType, routeFormat string, timeout, idleTimeout int32, routeConfig map[string]string, headers map[string][]string) (*routes.PostAppsAppRoutesOK, error) {
	cfg := &routes.PostAppsAppRoutesParams{
		App: appName,
		Body: &models.RouteWrapper{
			Route: &models.Route{
				Config:      routeConfig,
				Headers:     headers,
				Image:       image,
				Path:        routePath,
				Type:        routeType,
				Format:      routeFormat,
				Timeout:     &timeout,
				IDLETimeout: &idleTimeout,
			},
		},
		Context: ctx,
	}
	ok, err := fnclient.Routes.PostAppsAppRoutes(cfg)
	if err == nil {
		approutesLock.Lock()
		r, got := appsandroutes[appName]
		if got {
			appsandroutes[appName] = append(r, routePath)
		} else {
			appsandroutes[appName] = []string{routePath}
		}
		approutesLock.Unlock()
	}
	return ok, err

}

func CreateRoute(t *testing.T, ctx context.Context, fnclient *client.Fn, appName, routePath, image, routeType, routeFormat string, timeout, idleTimeout int32, routeConfig map[string]string, headers map[string][]string) {
	routeResponse, err := createRoute(ctx, fnclient, appName, image, routePath, routeType, routeFormat, timeout, idleTimeout, routeConfig, headers)
	CheckRouteResponseError(t, err)

	assertRouteFields(t, routeResponse.Payload.Route, routePath, image, routeType, routeFormat)
}

func deleteRoute(ctx context.Context, fnclient *client.Fn, appName, routePath string) (*routes.DeleteAppsAppRoutesRouteOK, error) {
	cfg := &routes.DeleteAppsAppRoutesRouteParams{
		App:     appName,
		Route:   routePath,
		Context: ctx,
	}

	return fnclient.Routes.DeleteAppsAppRoutesRoute(cfg)
}

func DeleteRoute(t *testing.T, ctx context.Context, fnclient *client.Fn, appName, routePath string) {
	_, err := deleteRoute(ctx, fnclient, appName, routePath)
	CheckRouteResponseError(t, err)
}

func ListRoutes(t *testing.T, ctx context.Context, fnclient *client.Fn, appName string) []*models.Route {
	cfg := &routes.GetAppsAppRoutesParams{
		App:     appName,
		Context: ctx,
	}

	routesResponse, err := fnclient.Routes.GetAppsAppRoutes(cfg)
	CheckRouteResponseError(t, err)
	return routesResponse.Payload.Routes
}

func GetRoute(t *testing.T, ctx context.Context, fnclient *client.Fn, appName, routePath string) *models.Route {
	cfg := &routes.GetAppsAppRoutesRouteParams{
		App:     appName,
		Route:   routePath,
		Context: ctx,
	}

	routeResponse, err := fnclient.Routes.GetAppsAppRoutesRoute(cfg)
	CheckRouteResponseError(t, err)
	return routeResponse.Payload.Route
}

func UpdateRoute(t *testing.T, ctx context.Context, fnclient *client.Fn, appName, routePath, image, routeType, format string, memory uint64, routeConfig map[string]string, headers map[string][]string, newRoutePath string) (*routes.PatchAppsAppRoutesRouteOK, error) {

	routeObject := GetRoute(t, ctx, fnclient, appName, routePath)
	if routeObject.Config == nil {
		routeObject.Config = map[string]string{}
	}

	if routeObject.Headers == nil {
		routeObject.Headers = map[string][]string{}
	}

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

func DeployRoute(t *testing.T, ctx context.Context, fnclient *client.Fn, appName, routePath, image, routeType, routeFormat string, routeConfig map[string]string, headers map[string][]string) *models.Route {
	cfg := &routes.PutAppsAppRoutesRouteParams{
		App:     appName,
		Context: ctx,
		Route:   routePath,
		Body: &models.RouteWrapper{
			Route: &models.Route{
				Config:  routeConfig,
				Headers: headers,
				Image:   image,
				Path:    routePath,
				Type:    routeType,
				Format:  routeFormat,
			},
		},
	}

	route, err := fnclient.Routes.PutAppsAppRoutesRoute(cfg)
	CheckRouteResponseError(t, err)
	return route.Payload.Route
}
