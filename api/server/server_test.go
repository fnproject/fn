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

	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
	"github.com/gin-gonic/gin"
)

var tmpDatastoreTests = "/tmp/func_test_datastore.db"

func testServer(ds models.Datastore, mq models.MessageQueue, logDB models.LogStore, rnr agent.Agent, nodeType serverNodeType) *Server {
	ctx := context.Background()

	s := &Server{
		Agent:     rnr,
		Router:    gin.New(),
		Datastore: ds,
		LogDB:     logDB,
		MQ:        mq,
		nodeType:  nodeType,
	}

	r := s.Router
	r.Use(gin.Logger())

	s.Router.Use(loggerWrap)
	s.bindHandlers(ctx)
	return s
}

func routerRequest(t *testing.T, router *gin.Engine, method, path string, body io.Reader) (*http.Request, *httptest.ResponseRecorder) {
	req, err := http.NewRequest(method, "http://127.0.0.1:8080"+path, body)
	if err != nil {
		t.Fatalf("Test: Could not create %s request to %s: %v", method, path, err)
	}
	return routerRequest2(t, router, req)
}

func routerRequest2(t *testing.T, router *gin.Engine, req *http.Request) (*http.Request, *httptest.ResponseRecorder) {

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

func prepareDB(ctx context.Context, t *testing.T) (models.Datastore, models.LogStore, func()) {
	os.Remove(tmpDatastoreTests)
	ds, err := datastore.New("sqlite3://" + tmpDatastoreTests)
	if err != nil {
		t.Fatalf("Error when creating datastore: %s", err)
	}
	logDB := ds
	return ds, logDB, func() {
		os.Remove(tmpDatastoreTests)
	}
}

func TestFullStack(t *testing.T) {
	ctx := context.Background()
	buf := setLogBuffer()
	ds, logDB, close := prepareDB(ctx, t)
	defer close()

	rnr, rnrcancel := testRunner(t, ds)
	defer rnrcancel()

	srv := testServer(ds, &mqs.Mock{}, logDB, rnr, nodeTypeFull)

	for _, test := range []struct {
		name              string
		method            string
		path              string
		body              string
		expectedCode      int
		expectedCacheSize int // TODO kill me
	}{
		{"create my app", "POST", "/v1/apps", `{ "app": { "name": "myapp" } }`, http.StatusOK, 0},
		{"list apps", "GET", "/v1/apps", ``, http.StatusOK, 0},
		{"get app", "GET", "/v1/apps/myapp", ``, http.StatusOK, 0},
		// NOTE: cache is lazy, loads when a request comes in for the route, not when added
		{"add myroute", "POST", "/v1/apps/myapp/routes", `{ "route": { "name": "myroute", "path": "/myroute", "image": "fnproject/hello", "type": "sync" } }`, http.StatusOK, 0},
		{"add myroute2", "POST", "/v1/apps/myapp/routes", `{ "route": { "name": "myroute2", "path": "/myroute2", "image": "fnproject/error", "type": "sync"  } }`, http.StatusOK, 0},
		{"get myroute", "GET", "/v1/apps/myapp/routes/myroute", ``, http.StatusOK, 0},
		{"get myroute2", "GET", "/v1/apps/myapp/routes/myroute2", ``, http.StatusOK, 0},
		{"get all routes", "GET", "/v1/apps/myapp/routes", ``, http.StatusOK, 0},
		{"execute myroute", "POST", "/r/myapp/myroute", `{ "name": "Teste" }`, http.StatusOK, 1},
		{"execute myroute2", "POST", "/r/myapp/myroute2", `{ "name": "Teste" }`, http.StatusBadGateway, 2},
		{"get myroute2", "GET", "/v1/apps/myapp/routes/myroute2", ``, http.StatusOK, 2},
		{"delete myroute", "DELETE", "/v1/apps/myapp/routes/myroute", ``, http.StatusOK, 1},
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
	}
}

func TestRunnerNode(t *testing.T) {
	ctx := context.Background()
	buf := setLogBuffer()
	ds, logDB, close := prepareDB(ctx, t)
	defer close()

	rnr, rnrcancel := testRunner(t, ds)
	defer rnrcancel()

	// Add route with an API server using the same DB
	{
		apiServer := testServer(ds, &mqs.Mock{}, logDB, rnr, nodeTypeAPI)
		_, rec := routerRequest(t, apiServer.Router, "POST", "/v1/apps/myapp/routes", bytes.NewBuffer([]byte(`{ "route": { "name": "myroute", "path": "/myroute", "image": "fnproject/hello", "type": "sync" } }`)))
		if rec.Code != http.StatusOK {
			t.Errorf("Expected status code 200 when creating sync route, but got %d", rec.Code)
		}
		_, rec = routerRequest(t, apiServer.Router, "POST", "/v1/apps/myapp/routes", bytes.NewBuffer([]byte(`{ "route": { "name": "myasyncroute", "path": "/myasyncroute", "image": "fnproject/hello", "type": "async" } }`)))
		if rec.Code != http.StatusOK {
			t.Errorf("Expected status code 200 when creating async route, but got %d", rec.Code)
		}
	}

	srv := testServer(ds, &mqs.Mock{}, logDB, rnr, nodeTypeRunner)

	for _, test := range []struct {
		name              string
		method            string
		path              string
		body              string
		expectedCode      int
		expectedCacheSize int // TODO kill me
	}{
		// Support sync and async API calls
		{"execute sync route succeeds", "POST", "/r/myapp/myroute", `{ "name": "Teste" }`, http.StatusOK, 1},
		{"execute async route succeeds", "POST", "/r/myapp/myasyncroute", `{ "name": "Teste" }`, http.StatusAccepted, 1},

		// All other API functions should not be available on runner nodes
		{"create app not found", "POST", "/v1/apps", `{ "app": { "name": "myapp" } }`, http.StatusNotFound, 0},
		{"list apps not found", "GET", "/v1/apps", ``, http.StatusNotFound, 0},
		{"get app not found", "GET", "/v1/apps/myapp", ``, http.StatusNotFound, 0},

		{"add route not found", "POST", "/v1/apps/myapp/routes", `{ "route": { "name": "myroute", "path": "/myroute", "image": "fnproject/hello", "type": "sync" } }`, http.StatusNotFound, 0},
		{"get route not found", "GET", "/v1/apps/myapp/routes/myroute", ``, http.StatusNotFound, 0},
		{"get all routes not found", "GET", "/v1/apps/myapp/routes", ``, http.StatusNotFound, 0},
		{"delete app not found", "DELETE", "/v1/apps/myapp", ``, http.StatusNotFound, 0},
	} {
		_, rec := routerRequest(t, srv.Router, test.method, test.path, bytes.NewBuffer([]byte(test.body)))

		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Errorf("Test \"%s\": Expected status code to be %d but was %d",
				test.name, test.expectedCode, rec.Code)
		}
	}
}

func TestApiNode(t *testing.T) {
	ctx := context.Background()
	buf := setLogBuffer()
	ds, logDB, close := prepareDB(ctx, t)
	defer close()

	rnr, rnrcancel := testRunner(t, ds)
	defer rnrcancel()

	srv := testServer(ds, &mqs.Mock{}, logDB, rnr, nodeTypeAPI)

	for _, test := range []struct {
		name              string
		method            string
		path              string
		body              string
		expectedCode      int
		expectedCacheSize int // TODO kill me
	}{
		// All routes should be supported
		{"create my app", "POST", "/v1/apps", `{ "app": { "name": "myapp" } }`, http.StatusOK, 0},
		{"list apps", "GET", "/v1/apps", ``, http.StatusOK, 0},
		{"get app", "GET", "/v1/apps/myapp", ``, http.StatusOK, 0},

		{"add myroute", "POST", "/v1/apps/myapp/routes", `{ "route": { "name": "myroute", "path": "/myroute", "image": "fnproject/hello", "type": "sync" } }`, http.StatusOK, 0},
		{"add myroute2", "POST", "/v1/apps/myapp/routes", `{ "route": { "name": "myroute2", "path": "/myroute2", "image": "fnproject/error", "type": "sync"  } }`, http.StatusOK, 0},
		{"add myasyncroute", "POST", "/v1/apps/myapp/routes", `{ "route": { "name": "myasyncroute", "path": "/myasyncroute", "image": "fnproject/hello", "type": "async" } }`, http.StatusOK, 0},
		{"get myroute", "GET", "/v1/apps/myapp/routes/myroute", ``, http.StatusOK, 0},
		{"get myroute2", "GET", "/v1/apps/myapp/routes/myroute2", ``, http.StatusOK, 0},
		{"get all routes", "GET", "/v1/apps/myapp/routes", ``, http.StatusOK, 0},

		// Don't support calling sync
		{"execute myroute", "POST", "/r/myapp/myroute", `{ "name": "Teste" }`, http.StatusBadRequest, 1},
		{"execute myroute2", "POST", "/r/myapp/myroute2", `{ "name": "Teste" }`, http.StatusBadRequest, 2},

		// Do support calling async
		{"execute myasyncroute", "POST", "/r/myapp/myasyncroute", `{ "name": "Teste" }`, http.StatusAccepted, 1},

		{"get myroute2", "GET", "/v1/apps/myapp/routes/myroute2", ``, http.StatusOK, 2},
		{"delete myroute", "DELETE", "/v1/apps/myapp/routes/myroute", ``, http.StatusOK, 1},
		{"delete myroute2", "DELETE", "/v1/apps/myapp/routes/myroute2", ``, http.StatusOK, 0},
		{"delete app (success)", "DELETE", "/v1/apps/myapp", ``, http.StatusOK, 0},
		{"get deleted app", "GET", "/v1/apps/myapp", ``, http.StatusNotFound, 0},
		{"get deleted route on deleted app", "GET", "/v1/apps/myapp/routes/myroute", ``, http.StatusNotFound, 0},
	} {
		_, rec := routerRequest(t, srv.Router, test.method, test.path, bytes.NewBuffer([]byte(test.body)))

		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Errorf("Test \"%s\": Expected status code to be %d but was %d",
				test.name, test.expectedCode, rec.Code)
		}
	}
}
