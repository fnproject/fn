package tests

import (
	"reflect"
	"strings"
	"testing"
	"time"

	fnTest "github.com/fnproject/fn/test"
	"github.com/fnproject/fn_go/client/apps"
)

func TestApps(t *testing.T) {

	t.Run("delete-app-not-found-test", func(t *testing.T) {
		t.Parallel()
		s := fnTest.SetupDefaultSuite()
		cfg := &apps.DeleteAppsAppParams{
			App:     "missing-app",
			Context: s.Context,
		}
		cfg.WithTimeout(time.Second * 60)
		_, err := s.Client.Apps.DeleteAppsApp(cfg)
		if err == nil {
			t.Errorf("Error during app delete: we should get HTTP 404, but got: %s", err.Error())
		}
	})

	t.Run("app-not-found-test", func(t *testing.T) {
		t.Parallel()
		s := fnTest.SetupDefaultSuite()
		cfg := &apps.GetAppsAppParams{
			App:     "missing-app",
			Context: s.Context,
		}
		cfg.WithTimeout(time.Second * 60)
		_, err := s.Client.Apps.GetAppsApp(cfg)
		fnTest.CheckAppResponseError(t, err)
	})

	t.Run("create-app-and-delete-no-config-test", func(t *testing.T) {
		t.Parallel()
		s := fnTest.SetupDefaultSuite()
		fnTest.CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		fnTest.DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("create-app-with-config-test", func(t *testing.T) {
		t.Parallel()
		s := fnTest.SetupDefaultSuite()
		fnTest.CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{"A": "a"})
		fnTest.DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("inspect-app-with-config-test", func(t *testing.T) {
		t.Parallel()
		s := fnTest.SetupDefaultSuite()
		fnTest.CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{"A": "a"})
		app := fnTest.GetApp(t, s.Context, s.Client, s.AppName)
		val, ok := app.Config["A"]
		if !ok {
			t.Error("Error during app config inspect: config map misses required entity `A` with value `a`.")
		}
		if !strings.Contains("a", val) {
			t.Errorf("App config value is different. Expected: `a`. Actual %v", val)
		}
		fnTest.DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("patch-app-with-exact-same-config-data", func(t *testing.T) {
		t.Parallel()
		s := fnTest.SetupDefaultSuite()
		config := map[string]string{
			"A": "a",
		}

		appUpdatePayload := fnTest.CreateUpdateApp(t, s.Context, s.Client, s.AppName, config)
		_, ok := appUpdatePayload.Payload.App.Config["A"]
		if !ok {
			t.Error("Error during app update: config map misses required entity `A` with value `a`.")
		}

		fnTest.DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("patch-override-app-config", func(t *testing.T) {
		t.Parallel()
		s := fnTest.SetupDefaultSuite()
		config := map[string]string{
			"A": "b",
		}
		appPayload := fnTest.CreateUpdateApp(t, s.Context, s.Client, s.AppName, config)
		val, ok := appPayload.Payload.App.Config["A"]
		if !ok {
			t.Error("Error during app config inspect: config map misses required entity `A` with value `a`.")
		}
		if !strings.Contains("b", val) {
			t.Errorf("App config value is different. Expected: `b`. Actual %v", val)
		}
		fnTest.DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("patch-add-app-config", func(t *testing.T) {
		t.Parallel()
		s := fnTest.SetupDefaultSuite()
		config := map[string]string{
			"B": "b",
		}
		appPayload := fnTest.CreateUpdateApp(t, s.Context, s.Client, s.AppName, config)
		val, ok := appPayload.Payload.App.Config["B"]
		if !ok {
			t.Error("Error during app config inspect: config map misses required entity `B` with value `b`.")
		}
		if !strings.Contains("b", val) {
			t.Errorf("App config value is different. Expected: `b`. Actual %v", val)
		}
		fnTest.DeleteApp(t, s.Context, s.Client, s.AppName)
	})

	t.Run("crete-app-duplicate", func(t *testing.T) {
		t.Parallel()
		s := fnTest.SetupDefaultSuite()
		fnTest.CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		_, err := fnTest.CreateAppNoAssert(s.Context, s.Client, s.AppName, map[string]string{})
		if reflect.TypeOf(err) != reflect.TypeOf(apps.NewPostAppsConflict()) {
			fnTest.CheckAppResponseError(t, err)
		}
		fnTest.DeleteApp(t, s.Context, s.Client, s.AppName)
	})
}
