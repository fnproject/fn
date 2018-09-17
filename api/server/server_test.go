package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"

	"github.com/fnproject/fn/api/agent"
	_ "github.com/fnproject/fn/api/agent/drivers/docker"
	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/datastore/sql"
	_ "github.com/fnproject/fn/api/datastore/sql/sqlite"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
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
