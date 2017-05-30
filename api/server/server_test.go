package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"gitlab-odx.oracle.com/odx/functions/api/datastore"
	"gitlab-odx.oracle.com/odx/functions/api/models"
	"gitlab-odx.oracle.com/odx/functions/api/mqs"
	"gitlab-odx.oracle.com/odx/functions/api/runner"
	"gitlab-odx.oracle.com/odx/functions/api/runner/task"
	"gitlab-odx.oracle.com/odx/functions/api/server/internal/routecache"
)

var tmpBolt = "/tmp/func_test_bolt.db"

func testServer(ds models.Datastore, mq models.MessageQueue, rnr *runner.Runner, tasks chan task.Request) *Server {
	ctx := context.Background()

	s := &Server{
		Runner:    rnr,
		Router:    gin.New(),
		Datastore: ds,
		MQ:        mq,
		tasks:     tasks,
		Enqueue:   DefaultEnqueue,
		hotroutes: routecache.New(2),
	}

	r := s.Router
	r.Use(gin.Logger())

	s.Router.Use(prepareMiddleware(ctx))
	s.bindHandlers(ctx)
	return s
}

func routerRequest(t *testing.T, router *gin.Engine, method, path string, body io.Reader) (*http.Request, *httptest.ResponseRecorder) {
	req, err := http.NewRequest(method, "http://127.0.0.1:8080"+path, body)
	if err != nil {
		t.Fatalf("Test: Could not create %s request to %s: %v", method, path, err)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return req, rec
}

func newRouterRequest(t *testing.T, method, path string, body io.Reader) (*http.Request, *httptest.ResponseRecorder) {
	req, err := http.NewRequest(method, "http://127.0.0.1:8080"+path, body)
	if err != nil {
		t.Fatalf("Test: Could not create %s request to %s: %v", method, path, err)
	}

	rec := httptest.NewRecorder()

	return req, rec
}

func getErrorResponse(t *testing.T, rec *httptest.ResponseRecorder) models.Error {
	respBody, err := ioutil.ReadAll(rec.Body)
	if err != nil {
		t.Error("Test: Expected not empty response body")
	}

	var errResp models.Error
	err = json.Unmarshal(respBody, &errResp)
	if err != nil {
		t.Error("Test: Expected response body to be a valid models.Error object")
	}

	return errResp
}

func prepareBolt(t *testing.T) (models.Datastore, func()) {
	os.Remove(tmpBolt)
	ds, err := datastore.New("bolt://" + tmpBolt)
	if err != nil {
		t.Fatal("Error when creating datastore: %s", err)
	}
	return ds, func() {
		os.Remove(tmpBolt)
	}
}

func TestFullStack(t *testing.T) {
	buf := setLogBuffer()
	ds, closeBolt := prepareBolt(t)
	defer closeBolt()

	tasks := make(chan task.Request)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rnr, rnrcancel := testRunner(t)
	defer rnrcancel()

	go runner.StartWorkers(ctx, rnr, tasks)

	srv := testServer(ds, &mqs.Mock{}, rnr, tasks)
	srv.hotroutes = routecache.New(2)

	for _, test := range []struct {
		name              string
		method            string
		path              string
		body              string
		expectedCode      int
		expectedCacheSize int
	}{
		{"create my app", "POST", "/v1/apps", `{ "app": { "name": "myapp" } }`, http.StatusOK, 0},
		{"list apps", "GET", "/v1/apps", ``, http.StatusOK, 0},
		{"get app", "GET", "/v1/apps/myapp", ``, http.StatusOK, 0},
		{"add myroute", "POST", "/v1/apps/myapp/routes", `{ "route": { "name": "myroute", "path": "/myroute", "image": "funcy/hello" } }`, http.StatusOK, 1},
		{"add myroute2", "POST", "/v1/apps/myapp/routes", `{ "route": { "name": "myroute2", "path": "/myroute2", "image": "funcy/error" } }`, http.StatusOK, 2},
		{"get myroute", "GET", "/v1/apps/myapp/routes/myroute", ``, http.StatusOK, 2},
		{"get myroute2", "GET", "/v1/apps/myapp/routes/myroute2", ``, http.StatusOK, 2},
		{"get all routes", "GET", "/v1/apps/myapp/routes", ``, http.StatusOK, 2},
		{"execute myroute", "POST", "/r/myapp/myroute", `{ "name": "Teste" }`, http.StatusOK, 2},
		{"execute myroute2", "POST", "/r/myapp/myroute2", `{ "name": "Teste" }`, http.StatusInternalServerError, 2},
		{"delete myroute", "DELETE", "/v1/apps/myapp/routes/myroute", ``, http.StatusOK, 1},
		{"delete app (fail)", "DELETE", "/v1/apps/myapp", ``, http.StatusBadRequest, 1},
		{"delete myroute2", "DELETE", "/v1/apps/myapp/routes/myroute2", ``, http.StatusOK, 0},
		{"delete app (success)", "DELETE", "/v1/apps/myapp", ``, http.StatusOK, 0},
		{"get deleted app", "GET", "/v1/apps/myapp", ``, http.StatusNotFound, 0},
		{"get deleteds route on deleted app", "GET", "/v1/apps/myapp/routes/myroute", ``, http.StatusNotFound, 0},
	} {
		_, rec := routerRequest(t, srv.Router, test.method, test.path, bytes.NewBuffer([]byte(test.body)))

		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Errorf("Test \"%s\": Expected status code to be %d but was %d",
				test.name, test.expectedCode, rec.Code)
		}
		if srv.hotroutes.Len() != test.expectedCacheSize {
			t.Log(buf.String())
			t.Errorf("Test \"%s\": Expected cache size to be %d but was %d",
				test.name, test.expectedCacheSize, srv.hotroutes.Len())
		}
	}
}
