package server

import (
	"bytes"
	"context"
	"fmt"
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
	}

	r := s.Router
	r.Use(gin.Logger())

	s.Router.Use(loggerWrap)
	s.bindHandlers(ctx)
	return r
}

func TestRouteRunnerAsyncExecution(t *testing.T) {
	ds := datastore.NewMockInit(
		[]*models.App{
			{Name: "myapp", Config: map[string]string{"app": "true"}},
		},
		[]*models.Route{
			{Type: "async", Path: "/myroute", AppName: "myapp", Image: "fnproject/hello", Config: map[string]string{"test": "true"}},
			{Type: "async", Path: "/myerror", AppName: "myapp", Image: "fnproject/error", Config: map[string]string{"test": "true"}},
			{Type: "async", Path: "/myroute/:param", AppName: "myapp", Image: "fnproject/hello", Config: map[string]string{"test": "true"}},
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

		fmt.Println("About to start router")
		rnr, cancel := testRunner(t, ds)
		router := testRouterAsync(ds, mq, rnr)

		fmt.Println("makeing requests")
		req, rec := newRouterRequest(t, "POST", test.path, body)
		for name, value := range test.headers {
			req.Header.Set(name, value[0])
		}
		fmt.Println("About to start router2")
		router.ServeHTTP(rec, req)
		fmt.Println("after servehttp")

		if rec.Code != test.expectedCode {
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}
		// TODO can test body and headers in the actual mq message w/ an agent that doesn't dequeue?
		// this just makes sure tasks are submitted (ok)...

		cancel()
	}
}
