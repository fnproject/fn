package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
)

func TestHybridEndpoints(t *testing.T) {
	buf := setLogBuffer()
	app := &models.App{ID: "app_id", Name: "myapp"}
	fn := &models.Fn{ID: "fn_id", AppID: app.ID}
	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Fn{fn},
	)

	srv := testServer(ds, nil /* TODO */, ServerTypeAPI)

	newCallBody := func() string {
		call := &models.Call{
			FnID: fn.ID,
			ID:   id.New().String(),
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

		{"post async call", "PUT", "/v2/runner/async", newCallBody(), http.StatusNoContent},

		// TODO this one only works if it's not the same as the first since update isn't hooked up
		{"finish call", "POST", "/v2/runner/finish", newCallBody(), http.StatusNoContent},

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
