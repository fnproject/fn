package server

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/iron-io/functions/api/datastore"
	"github.com/iron-io/functions/api/models"
)

func TestRouteRunnerGet(t *testing.T) {
	New(&datastore.Mock{}, &models.Config{})
	router := testRouter()

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{"/route", "", http.StatusNotFound, models.ErrRunnerRouteNotFound},
		{"/r/app/route", "", http.StatusNotFound, models.ErrRunnerRouteNotFound},
		{"/route?payload=test", "", http.StatusBadRequest, models.ErrInvalidJSON},
		{"/r/app/route?payload=test", "", http.StatusBadRequest, models.ErrInvalidJSON},
	} {
		_, rec := routerRequest(t, router, "GET", test.path, nil)

		if rec.Code != test.expectedCode {
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
				t.Errorf("Test %d: Expected error message to have `%s`",
					i, test.expectedError.Error())
			}
		}
	}
}

func TestRouteRunnerPost(t *testing.T) {
	New(&datastore.Mock{}, &models.Config{})
	router := testRouter()

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{"/route", `payload`, http.StatusBadRequest, models.ErrInvalidJSON},
		{"/r/app/route", `payload`, http.StatusBadRequest, models.ErrInvalidJSON},
		{"/route", `{ "payload": "" }`, http.StatusNotFound, models.ErrRunnerRouteNotFound},
		{"/r/app/route", `{ "payload": "" }`, http.StatusNotFound, models.ErrRunnerRouteNotFound},
	} {
		body := bytes.NewBuffer([]byte(test.body))
		_, rec := routerRequest(t, router, "POST", test.path, body)

		if rec.Code != test.expectedCode {
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
				t.Errorf("Test %d: Expected error message to have `%s`",
					i, test.expectedError.Error())
			}
		}
	}
}

func TestRouteRunnerExecution(t *testing.T) {
	New(&datastore.Mock{
		FakeRoutes: []*models.Route{
			{Path: "/myroute", Image: "iron/hello", Headers: map[string][]string{"X-Function": []string{"Test"}}},
			{Path: "/myerror", Image: "iron/error", Headers: map[string][]string{"X-Function": []string{"Test"}}},
		},
	}, &models.Config{})
	router := testRouter()

	for i, test := range []struct {
		path            string
		body            string
		expectedCode    int
		expectedHeaders map[string][]string
	}{
		{"/r/myapp/myroute", ``, http.StatusOK, map[string][]string{"X-Function": []string{"Test"}}},
		{"/r/myapp/myerror", ``, http.StatusInternalServerError, map[string][]string{"X-Function": []string{"Test"}}},
	} {
		body := bytes.NewBuffer([]byte(test.body))
		_, rec := routerRequest(t, router, "GET", test.path, body)

		if rec.Code != test.expectedCode {
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedHeaders != nil {
			for name, header := range test.expectedHeaders {
				if header[0] != rec.Header().Get(name) {
					t.Errorf("Test %d: Expected header `%s` to be %s but was %s",
						i, name, header[0], rec.Header().Get(name))
				}
			}
		}
	}
}
