package tests

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/funcy/functions_go/client/apps"
)

func TestApps(t *testing.T) {
	s := SetupDefaultSuite()

	t.Run("no-apps-found-test", func(t *testing.T) {
		cfg := &apps.GetAppsParams{
			Context: s.Context,
		}
		cfg.WithTimeout(time.Second * 60)
		appsPayload, err := s.Client.Apps.GetApps(cfg)
		CheckAppResponseError(t, err)
		// on this step we should not have any apps so far
		actualApps := appsPayload.Payload.Apps
		if len(actualApps) != 0 {
			t.Fatalf("Expected to see no apps, but found %v apps.", len(actualApps))
		}
		t.Logf("Test `%v` passed", t.Name())
	})

	t.Run("app-not-found-test", func(t *testing.T) {
		cfg := &apps.GetAppsAppParams{
			App:     "missing-app",
			Context: s.Context,
		}
		cfg.WithTimeout(time.Second * 60)
		_, err := s.Client.Apps.GetAppsApp(cfg)
		CheckAppResponseError(t, err)
		t.Logf("Test `%v` passed", t.Name())
	})

	t.Run("create-app-no-config-test", func(t *testing.T) {
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		t.Logf("Test `%v` passed.", t.Name())
	})

	t.Run("delete-app-no-config", func(t *testing.T) {
		DeleteApp(t, s.Context, s.Client, s.AppName)
		t.Logf("Test `%v` passed", t.Name())
	})

	t.Run("create-app-with-config-test", func(t *testing.T) {
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{"A": "a"})
		t.Logf("Test `%v` passed.", t.Name())
	})

	t.Run("inspect-app-with-config-test", func(t *testing.T) {
		cfg := &apps.GetAppsAppParams{
			Context: s.Context,
			App:     s.AppName,
		}
		appPayload, err := s.Client.Apps.GetAppsApp(cfg)
		CheckAppResponseError(t, err)
		appBody := appPayload.Payload.App
		val, ok := appBody.Config["A"]
		if !ok {
			t.Fatal("Error during app config inspect: config map misses required entity `A` with value `a`.")
		}
		if !strings.Contains("a", val) {
			t.Fatalf("App config value is different. Expected: `a`. Actual %v", val)
		}
		DeleteApp(t, s.Context, s.Client, s.AppName)
		t.Logf("Test `%v` passed.", t.Name())
	})

	t.Run("patch-override-app-config", func(t *testing.T) {
		config := map[string]string{
			"A": "b",
		}
		appPayload := UpdateApp(t, s.Context, s.Client, s.AppName, config)
		val, ok := appPayload.Payload.App.Config["A"]
		if !ok {
			t.Fatal("Error during app config inspect: config map misses required entity `A` with value `a`.")
		}
		if !strings.Contains("b", val) {
			t.Fatalf("App config value is different. Expected: `b`. Actual %v", val)
		}
		DeleteApp(t, s.Context, s.Client, s.AppName)
		t.Logf("Test `%v` passed.", t.Name())
	})

	t.Run("patch-add-app-config", func(t *testing.T) {
		config := map[string]string{
			"B": "b",
		}
		appPayload := UpdateApp(t, s.Context, s.Client, s.AppName, config)
		val, ok := appPayload.Payload.App.Config["B"]
		if !ok {
			t.Fatal("Error during app config inspect: config map misses required entity `B` with value `b`.")
		}
		if !strings.Contains("b", val) {
			t.Fatalf("App config value is different. Expected: `b`. Actual %v", val)
		}
		DeleteApp(t, s.Context, s.Client, s.AppName)
		t.Logf("Test `%v` passed.", t.Name())
	})

	t.Run("crete-app-duplicate", func(t *testing.T) {
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		_, err := CreateAppNoAssert(s.Context, s.Client, s.AppName, map[string]string{})
		if reflect.TypeOf(err) != reflect.TypeOf(apps.NewPostAppsConflict()) {
			CheckAppResponseError(t, err)
		}
		DeleteApp(t, s.Context, s.Client, s.AppName)
		t.Logf("Test `%v` passed.", t.Name())
	})
}
