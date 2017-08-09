package tests

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/funcy/functions_go/client/apps"
)

func TestApps(t *testing.T) {

	t.Run("app-not-found-test", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		cfg := &apps.GetAppsAppParams{
			App:     "missing-app",
			Context: s.Context,
		}
		cfg.WithTimeout(time.Second * 60)
		_, err := s.Client.Apps.GetAppsApp(cfg)
		CheckAppResponseError(t, err)
	})

	t.Run("create-app-and-delete-no-config-test", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("create-app-with-config-test", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{"A": "a"})
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("inspect-app-with-config-test", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{"A": "a"})
		cfg := &apps.GetAppsAppParams{
			Context: s.Context,
			App:     s.AppName,
		}
		appPayload, err := s.Client.Apps.GetAppsApp(cfg)
		CheckAppResponseError(t, err)
		appBody := appPayload.Payload.App
		val, ok := appBody.Config["A"]
		if !ok {
			t.Error("Error during app config inspect: config map misses required entity `A` with value `a`.")
		}
		if !strings.Contains("a", val) {
			t.Errorf("App config value is different. Expected: `a`. Actual %v", val)
		}
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("patch-app-with-exact-same-config-data", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		config := map[string]string{
			"A": "a",
		}

		appUpdatePayload := CreateUpdateApp(t, s.Context, s.Client, s.AppName, config)
		_, ok := appUpdatePayload.Payload.App.Config["A"]
		if !ok {
			t.Error("Error during app update: config map misses required entity `A` with value `a`.")
		}

		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("patch-override-app-config", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		config := map[string]string{
			"A": "b",
		}
		appPayload := CreateUpdateApp(t, s.Context, s.Client, s.AppName, config)
		val, ok := appPayload.Payload.App.Config["A"]
		if !ok {
			t.Error("Error during app config inspect: config map misses required entity `A` with value `a`.")
		}
		if !strings.Contains("b", val) {
			t.Errorf("App config value is different. Expected: `b`. Actual %v", val)
		}
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("patch-add-app-config", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		config := map[string]string{
			"B": "b",
		}
		appPayload := CreateUpdateApp(t, s.Context, s.Client, s.AppName, config)
		val, ok := appPayload.Payload.App.Config["B"]
		if !ok {
			t.Error("Error during app config inspect: config map misses required entity `B` with value `b`.")
		}
		if !strings.Contains("b", val) {
			t.Errorf("App config value is different. Expected: `b`. Actual %v", val)
		}
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("crete-app-duplicate", func(t *testing.T) {
		t.Parallel()
		s := SetupDefaultSuite()
		CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		_, err := CreateAppNoAssert(s.Context, s.Client, s.AppName, map[string]string{})
		if reflect.TypeOf(err) != reflect.TypeOf(apps.NewPostAppsConflict()) {
			CheckAppResponseError(t, err)
		}
		DeleteApp(t, s.Context, s.Client, s.AppName)
	})
}
