package tests

import (
	"context"
	"testing"
	"time"

	"github.com/funcy/functions_go/client"
	"github.com/funcy/functions_go/client/routes"
	"github.com/funcy/functions_go/models"
)

func CheckRouteResponseError(t *testing.T, err error) {
	if err != nil {
		switch err.(type) {

		case *routes.PostAppsAppRoutesDefault:
			msg := err.(*routes.PostAppsAppRoutesDefault).Payload.Error.Message
			code := err.(*routes.PostAppsAppRoutesDefault).Code()
			t.Errorf("Unexpected error occurred: %v. Status code: %v", msg, code)
		case *routes.PostAppsAppRoutesBadRequest:
			msg := err.(*routes.PostAppsAppRoutesBadRequest).Payload.Error.Message
			t.Errorf("Unexpected error occurred: %v.", msg)
		case *routes.PostAppsAppRoutesConflict:
			msg := err.(*routes.PostAppsAppRoutesConflict).Payload.Error.Message
			t.Errorf("Unexpected error occurred: %v.", msg)
		case *routes.GetAppsAppRoutesRouteNotFound:
			msg := err.(*routes.GetAppsAppRoutesRouteNotFound).Payload.Error.Message
			t.Errorf("Unexpected error occurred: %v.", msg)
		case *routes.GetAppsAppRoutesRouteDefault:
			msg := err.(*routes.GetAppsAppRoutesRouteDefault).Payload.Error.Message
			code := err.(*routes.GetAppsAppRoutesRouteDefault).Code()
			t.Errorf("Unexpected error occurred: %v. Status code: %v", msg, code)
		case *routes.DeleteAppsAppRoutesRouteNotFound:
			msg := err.(*routes.DeleteAppsAppRoutesRouteNotFound).Payload.Error.Message
			t.Errorf("Unexpected error occurred: %v.", msg)
		case *routes.DeleteAppsAppRoutesRouteDefault:
			msg := err.(*routes.DeleteAppsAppRoutesRouteDefault).Payload.Error.Message
			code := err.(*routes.DeleteAppsAppRoutesRouteDefault).Code()
			t.Errorf("Unexpected error occurred: %v. Status code: %v", msg, code)
		case *routes.GetAppsAppRoutesNotFound:
			msg := err.(*routes.GetAppsAppRoutesNotFound).Payload.Error.Message
			t.Errorf("Unexpected error occurred: %v.", msg)
		case *routes.GetAppsAppRoutesDefault:
			msg := err.(*routes.GetAppsAppRoutesDefault).Payload.Error.Message
			code := err.(*routes.GetAppsAppRoutesDefault).Code()
			t.Errorf("Unexpected error occurred: %v. Status code: %v", msg, code)
		case *routes.PatchAppsAppRoutesRouteBadRequest:
			msg := err.(*routes.PatchAppsAppRoutesRouteBadRequest).Payload.Error.Message
			t.Errorf("Unexpected error occurred: %v.", msg)
		case *routes.PatchAppsAppRoutesRouteNotFound:
			msg := err.(*routes.PatchAppsAppRoutesRouteNotFound).Payload.Error.Message
			t.Errorf("Unexpected error occurred: %v.", msg)
		case *routes.PatchAppsAppRoutesRouteDefault:
			msg := err.(*routes.PatchAppsAppRoutesRouteDefault).Payload.Error.Message
			code := err.(*routes.PatchAppsAppRoutesRouteDefault).Code()
			t.Errorf("Unexpected error occurred: %v. Status code: %v", msg, code)
		default:
			t.Errorf("Unable to determine type of error: %s", err)
		}
	}
}

func assertRouteFields(t *testing.T, routeObject *models.Route, path, image, routeType string) {

	rPath := routeObject.Path
	rImage := routeObject.Image
	rType := routeObject.Type
	rTimeout := *routeObject.Timeout
	rIdleTimeout := *routeObject.IDLETimeout
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

}

func createRoute(ctx context.Context, fnclient *client.Functions, appName, image, routePath, routeType string, routeConfig map[string]string, headers map[string][]string) (*routes.PostAppsAppRoutesOK, error) {
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

func CreateRoute(t *testing.T, ctx context.Context, fnclient *client.Functions, appName, routePath, image, routeType string, routeConfig map[string]string, headers map[string][]string) {
	routeResponse, err := createRoute(ctx, fnclient, appName, image, routePath, routeType, routeConfig, headers)
	CheckRouteResponseError(t, err)

	assertRouteFields(t, routeResponse.Payload.Route, routePath, image, routeType)
}

func deleteRoute(ctx context.Context, fnclient *client.Functions, appName, routePath string) (*routes.DeleteAppsAppRoutesRouteOK, error) {
	cfg := &routes.DeleteAppsAppRoutesRouteParams{
		App:     appName,
		Route:   routePath,
		Context: ctx,
	}
	cfg.WithTimeout(time.Second * 60)
	return fnclient.Routes.DeleteAppsAppRoutesRoute(cfg)
}

func DeleteRoute(t *testing.T, ctx context.Context, fnclient *client.Functions, appName, routePath string) {
	_, err := deleteRoute(ctx, fnclient, appName, routePath)
	CheckRouteResponseError(t, err)
}

func ListRoutes(t *testing.T, ctx context.Context, fnclient *client.Functions, appName string) []*models.Route {
	cfg := &routes.GetAppsAppRoutesParams{
		App:     appName,
		Context: ctx,
	}
	cfg.WithTimeout(time.Second * 60)
	routesResponse, err := fnclient.Routes.GetAppsAppRoutes(cfg)
	CheckRouteResponseError(t, err)
	return routesResponse.Payload.Routes
}

func GetRoute(t *testing.T, ctx context.Context, fnclient *client.Functions, appName, routePath string) *models.Route {
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

func UpdateRoute(t *testing.T, ctx context.Context, fnclient *client.Functions, appName, routePath, image, routeType, format string, memory uint64, routeConfig map[string]string, headers map[string][]string, newRoutePath string) (*routes.PatchAppsAppRoutesRouteOK, error) {

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
	cfg.WithTimeout(time.Second * 60)

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
