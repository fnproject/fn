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

func CheckAppResponseError(t *testing.T, e error) {
	if e != nil {
		switch err := e.(type) {
		case *apps.DeleteAppsAppDefault:
			t.Errorf("Unexpected error occurred: %v. Status code: %v Orig Location: %s", err.Payload.Error.Message, err.Code(), MyCaller())
			t.FailNow()
		case *apps.PostAppsDefault:
			t.Errorf("Unexpected error occurred: %v. Status code: %v Orig Location: %s", err.Payload.Error.Message, err.Code(), MyCaller())
			t.FailNow()
		case *apps.GetAppsAppNotFound:
			if !strings.Contains("App not found", err.Payload.Error.Message) {
				t.Errorf("Unexpected error occurred: %v Original Location: %s", err.Payload.Error.Message, MyCaller())
				t.FailNow()
			}
		case *apps.GetAppsAppDefault:
			t.Errorf("Unexpected error occurred: %v. Status code: %v Orig Location: %s", err.Payload.Error.Message, err.Code(), MyCaller())
			t.FailNow()
		case *apps.PatchAppsAppDefault:
			t.Errorf("Unexpected error occurred: %v. Status code: %v Orig Location: %s", err.Payload.Error.Message, err.Code(), MyCaller())
			t.FailNow()
		case *apps.PatchAppsAppNotFound:
			t.Errorf("Unexpected error occurred: %v. Original Location: %s", err.Payload.Error.Message, MyCaller())
			t.FailNow()
		case *apps.PatchAppsAppBadRequest:
			t.Errorf("Unexpected error occurred: %v. Original Location: %s", err.Payload.Error.Message, MyCaller())
			t.FailNow()
		default:
			t.Errorf("Unable to determine type of error: %s Original Location: %s", err, MyCaller())
			t.FailNow()
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
	ok, err := fnclient.Apps.PostApps(cfg)
	if err == nil {
		approutesLock.Lock()
		_, got := appsandroutes[appName]
		if !got {
			appsandroutes[appName] = []string{}
		}
		approutesLock.Unlock()
	}
	return ok, err
}

func CreateApp(t *testing.T, ctx context.Context, fnclient *client.Functions, appName string, config map[string]string) {
	appPayload, err := CreateAppNoAssert(ctx, fnclient, appName, config)
	CheckAppResponseError(t, err)
	if !strings.Contains(appName, appPayload.Payload.App.Name) {
		t.Errorf("App name mismatch.\nExpected: %v\nActual: %v",
			appName, appPayload.Payload.App.Name)
	}
}

func CreateUpdateApp(t *testing.T, ctx context.Context, fnclient *client.Functions, appName string, config map[string]string) *apps.PatchAppsAppOK {
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

	_, err := fnclient.Apps.DeleteAppsApp(cfg)
	CheckAppResponseError(t, err)
}

func GetApp(t *testing.T, ctx context.Context, fnclient *client.Functions, appName string) *models.App {
	cfg := &apps.GetAppsAppParams{
		App:     appName,
		Context: ctx,
	}

	app, err := fnclient.Apps.GetAppsApp(cfg)
	CheckAppResponseError(t, err)
	return app.Payload.App
}

func DeleteAppNoT(ctx context.Context, fnclient *client.Functions, appName string) {
	cfg := &apps.DeleteAppsAppParams{
		App:     appName,
		Context: ctx,
	}
	cfg.WithTimeout(time.Second * 60)
	fnclient.Apps.DeleteAppsApp(cfg)
}
