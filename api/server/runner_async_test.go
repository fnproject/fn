package server

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
	"github.com/gin-gonic/gin"
)

func testRouterAsync(ds models.Datastore, mq models.MessageQueue, rnr agent.Agent) *gin.Engine {
	ctx := context.Background()

	s := &Server{
		Agent:     rnr,
		Router:    gin.New(),
		Datastore: ds,
		MQ:        mq,
		nodeType:  ServerTypeFull,
	}

	r := s.Router
	r.Use(gin.Logger())

	s.Router.Use(loggerWrap)
	s.bindHandlers(ctx)
	return r
}

func TestRouteRunnerAsyncExecution(t *testing.T) {
	buf := setLogBuffer()

	ds := datastore.NewMockInit(
		[]*models.App{
			{Name: "myapp", Config: map[string]string{"app": "true"}},
		},
		[]*models.Route{
			{Type: "async", Path: "/myroute", AppName: "myapp", Image: "fnproject/hello", Config: map[string]string{"test": "true"}, Memory: 128, Timeout: 30, IdleTimeout: 30},
			{Type: "async", Path: "/myerror", AppName: "myapp", Image: "fnproject/error", Config: map[string]string{"test": "true"}, Memory: 128, Timeout: 30, IdleTimeout: 30},
			{Type: "async", Path: "/myroute/:param", AppName: "myapp", Image: "fnproject/hello", Config: map[string]string{"test": "true"}, Memory: 128, Timeout: 30, IdleTimeout: 30},
		}, nil,
	)
	mq := &mqs.Mock{}

	for i, test := range []struct {
		path         string
		body         string
		headers      map[string][]string
		expectedCode int
		expectedEnv  map[string]string
	}{
		{"/r/myapp/myroute", ``, map[string][]string{}, http.StatusAccepted, map[string]string{"TEST": "true", "APP": "true"}},
		// FIXME: this just hangs
		//{"/r/myapp/myroute/1", ``, map[string][]string{}, http.StatusAccepted, map[string]string{"TEST": "true", "APP": "true"}},
		{"/r/myapp/myerror", ``, map[string][]string{}, http.StatusAccepted, map[string]string{"TEST": "true", "APP": "true"}},
		{"/r/myapp/myroute", `{ "name": "test" }`, map[string][]string{}, http.StatusAccepted, map[string]string{"TEST": "true", "APP": "true"}},
		{
			"/r/myapp/myroute",
			``,
			map[string][]string{"X-Function": []string{"test"}},
			http.StatusAccepted,
			map[string]string{
				"TEST":              "true",
				"APP":               "true",
				"HEADER_X_FUNCTION": "test",
			},
		},
	} {
		body := bytes.NewBuffer([]byte(test.body))

		t.Log("About to start router")
		rnr, cancel := testRunner(t, ds)
		router := testRouterAsync(ds, mq, rnr)

		t.Log("making requests")
		req, rec := newRouterRequest(t, "POST", test.path, body)
		for name, value := range test.headers {
			req.Header.Set(name, value[0])
		}
		t.Log("About to start router2")
		router.ServeHTTP(rec, req)
		t.Log("after servehttp")

		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}
		// TODO can test body and headers in the actual mq message w/ an agent that doesn't dequeue?
		// this just makes sure tasks are submitted (ok)...

		cancel()
	}
}
