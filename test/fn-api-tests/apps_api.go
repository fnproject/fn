package tests

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/funcy/functions_go/client"
	"github.com/funcy/functions_go/client/apps"
	"github.com/funcy/functions_go/models"
)

func CheckAppResponseError(t *testing.T, err error) {
	if err != nil {
		switch err.(type) {

		case *apps.DeleteAppsAppDefault:
			msg := err.(*apps.DeleteAppsAppDefault).Payload.Error.Message
			code := err.(*apps.DeleteAppsAppDefault).Code()
			t.Errorf("Unexpected error occurred: %v. Status code: %v", msg, code)
		case *apps.PostAppsDefault:
			msg := err.(*apps.PostAppsDefault).Payload.Error.Message
			code := err.(*apps.PostAppsDefault).Code()
			t.Errorf("Unexpected error occurred: %v. Status code: %v", msg, code)
		case *apps.GetAppsAppNotFound:
			msg := err.(*apps.GetAppsAppNotFound).Payload.Error.Message
			if !strings.Contains("App not found", msg) {
				t.Errorf("Unexpected error occurred: %v", msg)
			}
		case *apps.GetAppsAppDefault:
			msg := err.(*apps.GetAppsAppDefault).Payload.Error.Message
			code := err.(*apps.GetAppsAppDefault).Code()
			t.Errorf("Unexpected error occurred: %v. Status code: %v", msg, code)
		case *apps.PatchAppsAppDefault:
			msg := err.(*apps.PatchAppsAppDefault).Payload.Error.Message
			code := err.(*apps.PatchAppsAppDefault).Code()
			t.Errorf("Unexpected error occurred: %v. Status code: %v", msg, code)
		case *apps.PatchAppsAppNotFound:
			msg := err.(*apps.PatchAppsAppNotFound).Payload.Error.Message
			t.Errorf("Unexpected error occurred: %v.", msg)
		case *apps.PatchAppsAppBadRequest:
			msg := err.(*apps.PatchAppsAppBadRequest).Payload.Error.Message
			t.Errorf("Unexpected error occurred: %v.", msg)
		default:
			t.Errorf("Unable to determine type of error: %s", err)
		}
	}

}

func CreateAppNoAssert(ctx context.Context, fnclient *client.Functions, appName string, config map[string]string) (*apps.PostAppsOK, error) {
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

func CreateApp(t *testing.T, ctx context.Context, fnclient *client.Functions, appName string, config map[string]string) {
	appPayload, err := CreateAppNoAssert(ctx, fnclient, appName, config)
	CheckAppResponseError(t, err)
	if !strings.Contains(appName, appPayload.Payload.App.Name) {
		t.Errorf("App name mismatch.\nExpected: %v\nActual: %v",
			appName, appPayload.Payload.App.Name)
	}
}

func UpdateApp(t *testing.T, ctx context.Context, fnclient *client.Functions, appName string, config map[string]string) *apps.PatchAppsAppOK {
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

func DeleteApp(t *testing.T, ctx context.Context, fnclient *client.Functions, appName string) {
	cfg := &apps.DeleteAppsAppParams{
		App:     appName,
		Context: ctx,
	}
	cfg.WithTimeout(time.Second * 60)
	_, err := fnclient.Apps.DeleteAppsApp(cfg)
	CheckAppResponseError(t, err)
}
