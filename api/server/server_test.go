package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/fnproject/fn/api/agent"
	_ "github.com/fnproject/fn/api/agent/drivers/docker"
	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/datastore/sql"
	_ "github.com/fnproject/fn/api/datastore/sql/sqlite"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
	"github.com/gin-gonic/gin"
)

var tmpDatastoreTests = "/tmp/func_test_datastore.db"

func testServer(ds models.Datastore, mq models.MessageQueue, logDB models.LogStore, rnr agent.Agent, nodeType NodeType, opts ...Option) *Server {
	return New(context.Background(), append(opts,
		WithLogLevel(getEnv(EnvLogLevel, DefaultLogLevel)),
		WithDatastore(ds),
		WithMQ(mq),
		WithLogstore(logDB),
		WithAgent(rnr),
		WithType(nodeType),
		WithTriggerAnnotator(NewRequestBasedTriggerAnnotator()),
		WithFnAnnotator(NewRequestBasedFnAnnotator()),
	)...)
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

func getV1ErrorResponse(t *testing.T, rec *httptest.ResponseRecorder) *models.ErrorWrapper {
	var err models.ErrorWrapper
	decodeErr := json.NewDecoder(rec.Body).Decode(&err)
	if decodeErr != nil {
		t.Error("Test: Expected not empty response body")
	}
	return &err
}

func getErrorResponse(t *testing.T, rec *httptest.ResponseRecorder) *models.Error {
	var err models.Error
	decodeErr := json.NewDecoder(rec.Body).Decode(&err)
	if decodeErr != nil {
		t.Error("Test: Expected not empty response body")
	}
	return &err
}

func prepareDB(ctx context.Context, t *testing.T) (models.Datastore, models.LogStore, func()) {
	os.Remove(tmpDatastoreTests)
	uri, err := url.Parse("sqlite3://" + tmpDatastoreTests)
	if err != nil {
		t.Fatal(err)
	}
	ss, err := sql.New(ctx, uri)
	if err != nil {
		t.Fatalf("Error when creating datastore: %s", err)
	}
	logDB := logs.Wrap(ss)
	ds := datastore.Wrap(ss)

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

	srv := testServer(ds, &mqs.Mock{}, logDB, rnr, ServerTypeFull, LimitRequestBody(32256))

	var bigbufa [32257]byte
	rand.Read(bigbufa[:])
	bigbuf := base64.StdEncoding.EncodeToString(bigbufa[:]) // this will be > bigbufa, but json compatible
	toobigerr := errors.New("Content-Length too large for this server")
	gatewayerr := errors.New("container exit code")

	for _, test := range []struct {
		name              string
		method            string
		path              string
		body              string
		expectedCode      int
		expectedCacheSize int // TODO kill me
		expectedError     error
	}{
		{"create my app", "POST", "/v1/apps", `{ "app": { "name": "myapp" } }`, http.StatusOK, 0, nil},
		{"list apps", "GET", "/v1/apps", ``, http.StatusOK, 0, nil},
		{"get app", "GET", "/v1/apps/myapp", ``, http.StatusOK, 0, nil},
		// NOTE: cache is lazy, loads when a request comes in for the route, not when added
		{"add myroute", "POST", "/v1/apps/myapp/routes", `{ "route": { "name": "myroute", "path": "/myroute", "image": "fnproject/fn-test-utils", "type": "sync" } }`, http.StatusOK, 0, nil},
		{"add myroute2", "POST", "/v1/apps/myapp/routes", `{ "route": { "name": "myroute2", "path": "/myroute2", "image": "fnproject/fn-test-utils", "type": "sync"  } }`, http.StatusOK, 0, nil},
		{"get myroute", "GET", "/v1/apps/myapp/routes/myroute", ``, http.StatusOK, 0, nil},
		{"get myroute2", "GET", "/v1/apps/myapp/routes/myroute2", ``, http.StatusOK, 0, nil},
		{"get all routes", "GET", "/v1/apps/myapp/routes", ``, http.StatusOK, 0, nil},
		{"execute myroute", "POST", "/r/myapp/myroute", `{ "echoContent": "Teste" }`, http.StatusOK, 1, nil},

		// fails
		{"execute myroute2", "POST", "/r/myapp/myroute2", `{"sleepTime": 0, "isDebug": true, "isCrash": true}`, http.StatusBadGateway, 2, gatewayerr},
		{"request body too large", "POST", "/r/myapp/myroute", bigbuf, http.StatusRequestEntityTooLarge, 0, toobigerr},

		{"get myroute2", "GET", "/v1/apps/myapp/routes/myroute2", ``, http.StatusOK, 2, nil},
		{"delete myroute", "DELETE", "/v1/apps/myapp/routes/myroute", ``, http.StatusOK, 1, nil},
		{"delete myroute2", "DELETE", "/v1/apps/myapp/routes/myroute2", ``, http.StatusOK, 0, nil},
		{"delete app (success)", "DELETE", "/v1/apps/myapp", ``, http.StatusOK, 0, nil},

		{"get deleted app", "GET", "/v1/apps/myapp", ``, http.StatusNotFound, 0, models.ErrAppsNotFound},
		{"get deleteds route on deleted app", "GET", "/v1/apps/myapp/routes/myroute", ``, http.StatusNotFound, 0, models.ErrAppsNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, rec := routerRequest(t, srv.Router, test.method, test.path, bytes.NewBuffer([]byte(test.body)))

			if rec.Code != test.expectedCode {
				t.Log(buf.String())
				t.Log(rec.Body.String())
				t.Errorf("Test \"%s\": Expected status code to be %d but was %d",
					test.name, test.expectedCode, rec.Code)
			}

			if rec.Code > 300 && test.expectedError == nil {
				t.Log(buf.String())
				t.Error("got error when not expected error", rec.Body.String())
			} else if test.expectedError != nil {
				if !strings.Contains(rec.Body.String(), test.expectedError.Error()) {
					t.Log(buf.String())
					t.Errorf("Test %s: Expected error message to have `%s`, but got `%s`",
						test.name, test.expectedError.Error(), rec.Body.String())
				}
			}
		})
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
		t.Run(test.name, func(t *testing.T) {
			_, rec := routerRequest(t, srv.Router, test.method, test.path, bytes.NewBuffer([]byte(test.body)))

			if rec.Code != test.expectedCode {
				t.Log(buf.String())
				t.Errorf("Test \"%s\": Expected status code to be %d but was %d",
					test.name, test.expectedCode, rec.Code)
			}
		})

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
