package agent

import (
	"context"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestCallConfigurationFromRouteAndEvent(t *testing.T) {
	appName := "myapp"
	path := "/"
	image := "fnproject/fn-test-utils"
	const timeout = 1
	const idleTimeout = 20
	const memory = 256
	typ := "sync"
	format := "default"

	cfg := models.Config{"APP_VAR": "FOO"}
	rCfg := models.Config{"ROUTE_VAR": "BAR"}

	app := &models.App{ID: "app_id", Name: appName, Config: cfg}
	route := &models.Route{
		AppID:       app.ID,
		Config:      rCfg,
		Path:        path,
		Image:       image,
		Type:        typ,
		Format:      format,
		Timeout:     timeout,
		IdleTimeout: idleTimeout,
		Memory:      memory,
	}

	// TODO this shouldn't require a log store

	ls := logs.NewMock()
	a := New(NewDirectCallDataAccess(ls, new(mqs.Mock)))
	defer checkClose(t, a)

	ctx := context.Background()

	evt := testEvent()
	call, err := a.GetCall(ctx,
		FromRouteAndEvent(app, route, evt),
	)

	if err != nil {
		t.Fatal(err)
	}

	model := call.Model()

	// make sure the values are all set correctly
	if model.ID == "" {
		t.Fatal("model does not have id, GetCall should assign id")
	}
	if model.AppID != app.ID {
		t.Fatal("app ID mismatch", model.ID, app.ID)
	}
	if model.Path != path {
		t.Fatal("path mismatch", model.Path, path)
	}
	if model.Image != image {
		t.Fatal("image mismatch", model.Image, image)
	}
	if model.Type != "sync" {
		t.Fatal("route type mismatch", model.Type)
	}
	if model.Priority == nil {
		t.Fatal("GetCall should make priority non-nil so that async works because for whatever reason some clowns plumbed it all over the mqs even though the user can't specify it gg")
	}
	if model.Timeout != timeout {
		t.Fatal("timeout mismatch", model.Timeout, timeout)
	}
	if model.IdleTimeout != idleTimeout {
		t.Fatal("idle timeout mismatch", model.IdleTimeout, idleTimeout)
	}
	if time.Time(model.CreatedAt).IsZero() {
		t.Fatal("GetCall should stamp CreatedAt, got nil timestamp")
	}

	if model.InputEvent == nil {
		t.Fatal("No input event on call")

	}

	cid, err := model.InputEvent.GetCallID()
	if err != nil {
		t.Fatalf("Failed to get call ID from event")
	}
	if cid != model.ID {
		t.Fatalf("call ID does not match model ")
	}

	expectedConfig := map[string]string{
		"FN_FORMAT":   format,
		"FN_APP_NAME": appName,
		"FN_PATH":     path,
		"FN_MEMORY":   strconv.Itoa(memory),
		"FN_TYPE":     typ,
		"APP_VAR":     "FOO",
		"ROUTE_VAR":   "BAR",
	}

	for k, v := range expectedConfig {
		if v2 := model.Config[k]; v2 != v {
			t.Fatal("config mismatch", k, v, v2, model.Config)
		}
		delete(expectedConfig, k)
	}

	if len(expectedConfig) > 0 {
		t.Fatal("got extra vars in config set, add me to tests ;)", expectedConfig)
	}

	// TODO check response writer for route headers
}

func TestCallConfigurationFromModel(t *testing.T) {
	app := &models.App{Name: "myapp"}

	path := "/"
	image := "fnproject/fn-test-utils"
	const timeout = 1
	const idleTimeout = 20
	const memory = 256
	CPUs := models.MilliCPUs(1000)
	typ := "sync"
	format := "default"
	cfg := models.Config{
		"FN_FORMAT":   format,
		"FN_APP_NAME": app.Name,
		"FN_PATH":     path,
		"FN_MEMORY":   strconv.Itoa(memory),
		"FN_CPUS":     CPUs.String(),
		"FN_TYPE":     typ,
		"APP_VAR":     "FOO",
		"ROUTE_VAR":   "BAR",
	}

	ctx := context.Background()

	evt := testEvent()
	cm := &models.Call{
		ID:          id.New().String(),
		AppID:       app.ID,
		Config:      cfg,
		Path:        path,
		Image:       image,
		Type:        typ,
		Format:      format,
		Timeout:     timeout,
		IdleTimeout: idleTimeout,
		Memory:      memory,
		CPUs:        CPUs,
		InputEvent:  evt,
	}

	// FromModel doesn't need a datastore, for now...
	ls := logs.NewMock()

	a := New(NewDirectCallDataAccess(ls, new(mqs.Mock)))
	defer checkClose(t, a)

	callI, err := a.GetCall(ctx, FromModel(cm))
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(callI.Model(), cm) {
		t.Fatal("call did not match model")
	}

}
