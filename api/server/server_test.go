package server

import (
	"bytes"
	"net/http"
	"os"
	"testing"

	"github.com/iron-io/functions/api/datastore"
	"github.com/iron-io/functions/api/models"
)

var tmpBolt = "/tmp/func_test_bolt.db"

func prepareBolt(t *testing.T) (models.Datastore, func()) {
	ds, err := datastore.New("bolt://" + tmpBolt)
	if err != nil {
		t.Fatal("Error when creating datastore: %s", err)
	}
	return ds, func() {
		os.Remove(tmpBolt)
	}
}

func TestFullStack(t *testing.T) {
	ds, close := prepareBolt(t)
	defer close()

	New(&models.Config{}, ds, testRunner(t))
	router := testRouter()

	for i, test := range []struct {
		method       string
		path         string
		body         string
		expectedCode int
	}{
		{"POST", "/v1/apps", `{ "app": { "name": "myapp" } }`, http.StatusCreated},
		{"GET", "/v1/apps", ``, http.StatusOK},
		{"GET", "/v1/apps/myapp", ``, http.StatusOK},
		{"POST", "/v1/apps/myapp/routes", `{ "route": { "name": "myroute", "path": "/myroute", "image": "iron/hello" } }`, http.StatusCreated},
		{"POST", "/v1/apps/myapp/routes", `{ "route": { "name": "myroute2", "path": "/myroute2", "image": "iron/error" } }`, http.StatusCreated},
		{"GET", "/v1/apps/myapp/routes/myroute", ``, http.StatusOK},
		{"GET", "/v1/apps/myapp/routes/myroute2", ``, http.StatusOK},
		{"GET", "/v1/apps/myapp/routes", ``, http.StatusOK},
		{"POST", "/r/myapp/myroute", `{ "name": "Teste" }`, http.StatusOK},
		{"POST", "/r/myapp/myroute2", `{ "name": "Teste" }`, http.StatusInternalServerError},
		{"DELETE", "/v1/apps/myapp/routes/myroute", ``, http.StatusOK},
		{"DELETE", "/v1/apps/myapp", ``, http.StatusOK},
		{"GET", "/v1/apps/myapp", ``, http.StatusNotFound},
		{"GET", "/v1/apps/myapp/routes/myroute", ``, http.StatusInternalServerError},
	} {
		_, rec := routerRequest(t, router, test.method, test.path, bytes.NewBuffer([]byte(test.body)))

		if rec.Code != test.expectedCode {
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}
	}

}
