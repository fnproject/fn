package server

import (
	"context"
	"testing"

	"github.com/iron-io/functions/api/datastore"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/mqs"
	"github.com/iron-io/functions/api/runner"
	"github.com/iron-io/functions/api/runner/task"
)

type testSpecialHandler struct{}

func (h *testSpecialHandler) Handle(c HandlerContext) error {
	c.Set("appName", "test")
	return nil
}

func TestSpecialHandlerSet(t *testing.T) {
	ctx := context.Background()

	tasks := make(chan task.Request)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rnr, cancelrnr := testRunner(t)
	defer cancelrnr()

	go runner.StartWorkers(ctx, rnr, tasks)

	s := New(ctx, &datastore.Mock{
		Apps: []*models.App{
			{Name: "test"},
		},
		Routes: []*models.Route{
			{Path: "/test", Image: "iron/hello", AppName: "test"},
		},
	}, &mqs.Mock{}, rnr, tasks, DefaultEnqueue)
	router := s.Router
	router.Use(prepareMiddleware(ctx))
	s.bindHandlers()
	s.AddSpecialHandler(&testSpecialHandler{})

	_, rec := routerRequest(t, router, "GET", "/test", nil)
	if rec.Code != 200 {
		t.Fatal("Test SpecialHandler: expected special handler to run functions successfully")
	}
}
