package server

import (
	"bytes"
	"context"
	"net/http"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/datastore"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/mqs"
	"github.com/iron-io/runner/common"
)

func testRouterAsync(enqueueFunc models.Enqueue) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())
	ctx := context.Background()
	r.Use(func(c *gin.Context) {
		ctx, _ := common.LoggerWithFields(ctx, extractFields(c))
		c.Set("ctx", ctx)
		c.Next()
	})
	bindHandlers(r,
		func(ctx *gin.Context) {
			handleRequest(ctx, enqueueFunc)
		},
		func(ctx *gin.Context) {})
	return r
}

func TestRouteRunnerAsyncExecution(t *testing.T) {
	New(&datastore.Mock{
		FakeApps: []*models.App{
			{Name: "myapp", Config: map[string]string{"app": "true"}},
		},
		FakeRoutes: []*models.Route{
			{Type: "async", Path: "/myroute", AppName: "myapp", Image: "iron/hello", Config: map[string]string{"test": "true"}},
			{Type: "async", Path: "/myerror", AppName: "myapp", Image: "iron/error", Config: map[string]string{"test": "true"}},
			{Type: "async", Path: "/myroute/:param", AppName: "myapp", Image: "iron/hello", Config: map[string]string{"test": "true"}},
		},
	}, &mqs.Mock{}, testRunner(t))

	for i, test := range []struct {
		path         string
		body         string
		headers      map[string][]string
		expectedCode int
		expectedEnv  map[string]string
	}{
		{"/r/myapp/myroute", ``, map[string][]string{}, http.StatusOK, map[string]string{"CONFIG_TEST": "true", "CONFIG_APP": "true"}},
		{
			"/r/myapp/myroute/1",
			``,
			map[string][]string{"X-Function": []string{"test"}},
			http.StatusOK,
			map[string]string{
				"CONFIG_TEST":       "true",
				"CONFIG_APP":        "true",
				"PARAM_PARAM":       "1",
				"HEADER_X_FUNCTION": "test",
			},
		},
		{"/r/myapp/myerror", ``, map[string][]string{}, http.StatusOK, map[string]string{"CONFIG_TEST": "true", "CONFIG_APP": "true"}},
		{"/r/myapp/myroute", `{ "name": "test" }`, map[string][]string{}, http.StatusOK, map[string]string{"CONFIG_TEST": "true", "CONFIG_APP": "true"}},
	} {
		body := bytes.NewBuffer([]byte(test.body))

		var wg sync.WaitGroup

		wg.Add(1)
		router := testRouterAsync(func(task *models.Task) (*models.Task, error) {
			if test.body != task.Payload {
				t.Errorf("Test %d: Expected task Payload to be the same as the test body", i)
			}

			if test.expectedEnv != nil {
				for name, value := range test.expectedEnv {
					if value != task.EnvVars[name] {
						t.Errorf("Test %d: Expected header `%s` to be `%s` but was `%s`",
							i, name, value, task.EnvVars[name])
					}
				}
			}

			wg.Done()
			return task, nil
		})

		req, rec := newRouterRequest(t, "POST", test.path, body)
		for name, value := range test.headers {
			req.Header.Set(name, value[0])
		}
		router.ServeHTTP(rec, req)

		if rec.Code != test.expectedCode {
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		wg.Wait()
	}
}
