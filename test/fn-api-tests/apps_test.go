package tests

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/fnproject/fn_go/client/apps"
)

func TestAppDeleteNotFound(t *testing.T) {
	t.Parallel()
	s := SetupDefaultSuite()
	cfg := &apps.DeleteAppsAppParams{
		App:     "missing-app",
		Context: s.Context,
	}
	cfg.WithTimeout(time.Second * 60)
	_, err := s.Client.Apps.DeleteAppsApp(cfg)
	if err == nil {
		t.Errorf("Error during app delete: we should get HTTP 404, but got: %s", err.Error())
	}
}

func TestAppGetNotFound(t *testing.T) {
	t.Parallel()
	s := SetupDefaultSuite()
	cfg := &apps.GetAppsAppParams{
		App:     "missing-app",
		Context: s.Context,
	}
	cfg.WithTimeout(time.Second * 60)
	_, err := s.Client.Apps.GetAppsApp(cfg)
	CheckAppResponseError(t, err)
}

func TestAppCreateNoConfigSuccess(t *testing.T) {
	t.Parallel()
	s := SetupDefaultSuite()
	CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
	DeleteApp(t, s.Context, s.Client, s.AppName)
}

func TestAppCreateWithConfigSuccess(t *testing.T) {
	t.Parallel()
	s := SetupDefaultSuite()
	CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{"A": "a"})
	DeleteApp(t, s.Context, s.Client, s.AppName)
}

func TestAppInsect(t *testing.T) {
	t.Parallel()
	s := SetupDefaultSuite()
	CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{"A": "a"})
	app := GetApp(t, s.Context, s.Client, s.AppName)
	val, ok := app.Config["A"]
	if !ok {
		t.Error("Error during app config inspect: config map misses required entity `A` with value `a`.")
	}
	if !strings.Contains("a", val) {
		t.Errorf("App config value is different. Expected: `a`. Actual %v", val)
	}
	DeleteApp(t, s.Context, s.Client, s.AppName)
}

func TestAppPatchSameConfig(t *testing.T) {
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
}

func TestAppPatchOverwriteConfig(t *testing.T) {
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
}

func TestAppsPatchConfigAddValue(t *testing.T) {
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
}

func TestAppDuplicate(t *testing.T) {
	t.Parallel()
	s := SetupDefaultSuite()
	CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
	_, err := CreateAppNoAssert(s.Context, s.Client, s.AppName, map[string]string{})
	if reflect.TypeOf(err) != reflect.TypeOf(apps.NewPostAppsConflict()) {
		CheckAppResponseError(t, err)
	}
	DeleteApp(t, s.Context, s.Client, s.AppName)
}
