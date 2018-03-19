package tests

import (
	"context"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/fnproject/fn_go/client"
	"github.com/fnproject/fn_go/client/apps"
	"github.com/fnproject/fn_go/models"
)

// PostApp creates an app and esures it is deleted on teardown  if it was created
func (s *TestHarness) PostApp(app *models.App) (*apps.PostAppsOK, error) {
	cfg := &apps.PostAppsParams{
		Body: &models.AppWrapper{
			App: app,
		},
		Context: s.Context,
	}
	ok, err := s.Client.Apps.PostApps(cfg)

	if err == nil {
		approutesLock.Lock()
		_, got := appsandroutes[ok.Payload.App.Name]
		if !got {
			appsandroutes[ok.Payload.App.Name] = []string{}
		}
		approutesLock.Unlock()
	}
	return ok, err
}

// GivenAppExists creates an app and ensures it is deleted on teardown, this fatals if the app is not created
func (s *TestHarness) GivenAppExists(t *testing.T, app *models.App) {

	appPayload, err := s.PostApp(app)
	if err != nil {
		{
			t.Fatalf("Failed to create app %v", app)
		}
	}
	if !strings.Contains(app.Name, appPayload.Payload.App.Name) {
		t.Fatalf("App name mismatch.\nExpected: %v\nActual: %v",
			app.Name, appPayload.Payload.App.Name)
	}
}

// AppMustExist fails the test if the specified app does not exist
func (s *TestHarness) AppMustExist(t *testing.T, appName string) *models.App {
	app, err := s.Client.Apps.GetAppsApp(&apps.GetAppsAppParams{
		App:     s.AppName,
		Context: s.Context,
	})
	if err != nil {
		t.Fatalf("Expected new route to create app  got %v", err)
		return nil
	}
	return app.Payload.App
}

func safeDeleteApp(ctx context.Context, fnclient *client.Fn, appName string) {
	cfg := &apps.DeleteAppsAppParams{
		App:     appName,
		Context: ctx,
	}
	cfg.WithTimeout(time.Second * 60)
	_, err := fnclient.Apps.DeleteAppsApp(cfg)
	if _, ok := err.(*apps.DeleteAppsAppNotFound); err != nil && !ok {
		log.Printf("Error cleaning up app %s: %v", appName, err)
	}
}
