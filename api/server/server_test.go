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
	"strconv"
	"strings"
	"testing"

	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
	"github.com/gin-gonic/gin"
)

var tmpDatastoreTests = "/tmp/func_test_datastore.db"

func testServer(ds models.Datastore, mq models.MessageQueue, logDB models.LogStore, rnr agent.Agent, nodeType ServerNodeType) *Server {
	return New(context.Background(),
		WithLogLevel(getEnv(EnvLogLevel, DefaultLogLevel)),
		WithDatastore(ds),
		WithMQ(mq),
		WithLogstore(logDB),
		WithAgent(rnr),
		WithType(nodeType),
	)
}

func createRequest(t *testing.T, method, path string, body io.Reader) *http.Request {

	bodyLen := int64(0)

	// HACK: derive content-length since protocol/http does not add content-length
	// if it's not present.
	if body != nil {
		buf := &bytes.Buffer{}
		nRead, err := io.Copy(buf, body)
		if err != nil {
			t.Fatalf("Test: Could not copy %s request body to %s: %v", method, path, err)
		}

		bodyLen = nRead
		body = buf
	}

	req, err := http.NewRequest(method, "http://127.0.0.1:8080"+path, body)
	if err != nil {
		t.Fatalf("Test: Could not create %s request to %s: %v", method, path, err)
	}

	if body != nil {
		req.ContentLength = bodyLen
		req.Header.Set("Content-Length", strconv.FormatInt(bodyLen, 10))
	}

	return req
}

func routerRequest(t *testing.T, router *gin.Engine, method, path string, body io.Reader) (*http.Request, *httptest.ResponseRecorder) {
	req := createRequest(t, method, path, body)
	return routerRequest2(t, router, req)
}

func routerRequest2(_ *testing.T, router *gin.Engine, req *http.Request) (*http.Request, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	rec.Body = new(bytes.Buffer)
	router.ServeHTTP(rec, req)
	return req, rec
}

func newRouterRequest(t *testing.T, method, path string, body io.Reader) (*http.Request, *httptest.ResponseRecorder) {
	req := createRequest(t, method, path, body)
	rec := httptest.NewRecorder()
	rec.Body = new(bytes.Buffer)
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
	ds, err := datastore.New(ctx, "sqlite3://"+tmpDatastoreTests)
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

	srv := testServer(ds, &mqs.Mock{}, logDB, rnr, ServerTypeFull)

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
		{"add myroute", "POST", "/v1/apps/myapp/routes", `{ "route": { "name": "myroute", "path": "/myroute", "image": "fnproject/fn-test-utils", "type": "sync" } }`, http.StatusOK, 0},
		{"add myroute2", "POST", "/v1/apps/myapp/routes", `{ "route": { "name": "myroute2", "path": "/myroute2", "image": "fnproject/fn-test-utils", "type": "sync"  } }`, http.StatusOK, 0},
		{"get myroute", "GET", "/v1/apps/myapp/routes/myroute", ``, http.StatusOK, 0},
		{"get myroute2", "GET", "/v1/apps/myapp/routes/myroute2", ``, http.StatusOK, 0},
		{"get all routes", "GET", "/v1/apps/myapp/routes", ``, http.StatusOK, 0},
		{"execute myroute", "POST", "/r/myapp/myroute", `{ "echoContent": "Teste" }`, http.StatusOK, 1},
		{"execute myroute2", "POST", "/r/myapp/myroute2", `{"sleepTime": 0, "isDebug": true, "isCrash": true}`, http.StatusBadGateway, 2},
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
			t.Log(rec.Body.String())
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
		apiServer := testServer(ds, &mqs.Mock{}, logDB, nil, ServerTypeAPI)
		_, rec := routerRequest(t, apiServer.Router, "POST", "/v1/apps/myapp/routes", bytes.NewBuffer([]byte(`{ "route": { "name": "myroute", "path": "/myroute", "image": "fnproject/fn-test-utils", "type": "sync" } }`)))
		if rec.Code != http.StatusOK {
			t.Errorf("Expected status code 200 when creating sync route, but got %d", rec.Code)
		}
		_, rec = routerRequest(t, apiServer.Router, "POST", "/v1/apps/myapp/routes", bytes.NewBuffer([]byte(`{ "route": { "name": "myasyncroute", "path": "/myasyncroute", "image": "fnproject/fn-test-utils", "type": "async" } }`)))
		if rec.Code != http.StatusOK {
			t.Errorf("Expected status code 200 when creating async route, but got %d", rec.Code)
		}
	}

	srv := testServer(ds, &mqs.Mock{}, logDB, rnr, ServerTypeRunner)

	for _, test := range []struct {
		name              string
		method            string
		path              string
		body              string
		expectedCode      int
		expectedCacheSize int // TODO kill me
	}{
		// Support sync and async API calls
		{"execute sync route succeeds", "POST", "/r/myapp/myroute", `{ "echoContent": "Teste" }`, http.StatusOK, 1},
		{"execute async route succeeds", "POST", "/r/myapp/myasyncroute", `{ "echoContent": "Teste" }`, http.StatusAccepted, 1},

		// All other API functions should not be available on runner nodes
		{"create app not found", "POST", "/v1/apps", `{ "app": { "name": "myapp" } }`, http.StatusBadRequest, 0},
		{"list apps not found", "GET", "/v1/apps", ``, http.StatusBadRequest, 0},
		{"get app not found", "GET", "/v1/apps/myapp", ``, http.StatusBadRequest, 0},

		{"add route not found", "POST", "/v1/apps/myapp/routes", `{ "route": { "name": "myroute", "path": "/myroute", "image": "fnproject/fn-test-utils", "type": "sync" } }`, http.StatusBadRequest, 0},
		{"get route not found", "GET", "/v1/apps/myapp/routes/myroute", ``, http.StatusBadRequest, 0},
		{"get all routes not found", "GET", "/v1/apps/myapp/routes", ``, http.StatusBadRequest, 0},
		{"delete app not found", "DELETE", "/v1/apps/myapp", ``, http.StatusBadRequest, 0},
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

	srv := testServer(ds, &mqs.Mock{}, logDB, nil, ServerTypeAPI)

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

		{"add myroute", "POST", "/v1/apps/myapp/routes", `{ "route": { "name": "myroute", "path": "/myroute", "image": "fnproject/fn-test-utils", "type": "sync" } }`, http.StatusOK, 0},
		{"add myroute2", "POST", "/v1/apps/myapp/routes", `{ "route": { "name": "myroute2", "path": "/myroute2", "image": "fnproject/fn-test-utils", "type": "sync"  } }`, http.StatusOK, 0},
		{"add myasyncroute", "POST", "/v1/apps/myapp/routes", `{ "route": { "name": "myasyncroute", "path": "/myasyncroute", "image": "fnproject/fn-test-utils", "type": "async" } }`, http.StatusOK, 0},
		{"get myroute", "GET", "/v1/apps/myapp/routes/myroute", ``, http.StatusOK, 0},
		{"get myroute2", "GET", "/v1/apps/myapp/routes/myroute2", ``, http.StatusOK, 0},
		{"get all routes", "GET", "/v1/apps/myapp/routes", ``, http.StatusOK, 0},

		// Don't support calling sync or async
		{"execute myroute", "POST", "/r/myapp/myroute", `{ "echoContent": "Teste" }`, http.StatusBadRequest, 1},
		{"execute myroute2", "POST", "/r/myapp/myroute2", `{ "echoContent": "Teste" }`, http.StatusBadRequest, 2},
		{"execute myasyncroute", "POST", "/r/myapp/myasyncroute", `{ "echoContent": "Teste" }`, http.StatusBadRequest, 1},

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

func TestHybridEndpoints(t *testing.T) {
	buf := setLogBuffer()
	app := &models.App{Name: "myapp"}
	app.SetDefaults()
	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Route{{
			AppID: app.ID,
			Path:  "yodawg",
		}}, nil,
	)

	logDB := logs.NewMock()

	srv := testServer(ds, &mqs.Mock{}, logDB, nil /* TODO */, ServerTypeAPI)

	newCallBody := func() string {
		call := &models.Call{
			ID:      id.New().String(),
			AppName: "myapp",
			AppID:   app.ID,
			Path:    "yodawg",
			// TODO ?
		}
		var b bytes.Buffer
		json.NewEncoder(&b).Encode(&call)
		return b.String()
	}

	for _, test := range []struct {
		name         string
		method       string
		path         string
		body         string
		expectedCode int
	}{
		// TODO change all these tests to just do an async task in normal order once plumbing is done

		{"post async call", "PUT", "/v1/runner/async", newCallBody(), http.StatusOK},

		// TODO this one only works if it's not the same as the first since update isn't hooked up
		{"finish call", "POST", "/v1/runner/finish", newCallBody(), http.StatusOK},

		// TODO these won't work until update works and the agent gets shut off
		//{"get async call", "GET", "/v1/runner/async", "", http.StatusOK},
		//{"start call", "POST", "/v1/runner/start", "TODO", http.StatusOK},
	} {
		_, rec := routerRequest(t, srv.Router, test.method, test.path, strings.NewReader(test.body))

		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Errorf("Test \"%s\": Expected status code to be %d but was %d",
				test.name, test.expectedCode, rec.Code)
		}
	}
}
