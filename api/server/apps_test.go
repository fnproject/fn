package server

import (
	"bytes"
	"log"
	"net/http"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
	"github.com/gin-gonic/gin"
)

func setLogBuffer() *bytes.Buffer {
	var buf bytes.Buffer
	buf.WriteByte('\n')
	logrus.SetOutput(&buf)
	gin.DefaultErrorWriter = &buf
	gin.DefaultWriter = &buf
	log.SetOutput(&buf)
	return &buf
}

func TestAppCreate(t *testing.T) {
	buf := setLogBuffer()
	for i, test := range []struct {
		mock          models.Datastore
		logDB         models.LogStore
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		// errors
		{datastore.NewMock(), logs.NewMock(), "/v1/apps", ``, http.StatusBadRequest, models.ErrInvalidJSON},
		{datastore.NewMock(), logs.NewMock(), "/v1/apps", `{}`, http.StatusBadRequest, models.ErrAppsMissingNew},
		{datastore.NewMock(), logs.NewMock(), "/v1/apps", `{ "name": "Test" }`, http.StatusBadRequest, models.ErrAppsMissingNew},
		{datastore.NewMock(), logs.NewMock(), "/v1/apps", `{ "app": { "name": "" } }`, http.StatusBadRequest, models.ErrAppsValidationMissingName},
		{datastore.NewMock(), logs.NewMock(), "/v1/apps", `{ "app": { "name": "1234567890123456789012345678901" } }`, http.StatusBadRequest, models.ErrAppsValidationTooLongName},
		{datastore.NewMock(), logs.NewMock(), "/v1/apps", `{ "app": { "name": "&&%@!#$#@$" } }`, http.StatusBadRequest, models.ErrAppsValidationInvalidName},
		{datastore.NewMock(), logs.NewMock(), "/v1/apps", `{ "app": { "name": "&&%@!#$#@$" } }`, http.StatusBadRequest, models.ErrAppsValidationInvalidName},

		// success
		{datastore.NewMock(), logs.NewMock(), "/v1/apps", `{ "app": { "name": "teste" } }`, http.StatusOK, nil},
	} {
		rnr, cancel := testRunner(t)
		srv := testServer(test.mock, &mqs.Mock{}, test.logDB, rnr)
		router := srv.Router

		body := bytes.NewBuffer([]byte(test.body))
		_, rec := routerRequest(t, router, "POST", test.path, body)

		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
				t.Log(buf.String())
				t.Errorf("Test %d: Expected error message to have `%s`",
					i, test.expectedError.Error())
			}
		}
		cancel()
	}
}

func TestAppDelete(t *testing.T) {
	buf := setLogBuffer()

	for i, test := range []struct {
		ds            models.Datastore
		logDB         models.LogStore
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{datastore.NewMock(), logs.NewMock(), "/v1/apps/myapp", "", http.StatusNotFound, nil},
		{datastore.NewMockInit(
			[]*models.App{{
				Name: "myapp",
			}}, nil, nil,
		), logs.NewMock(), "/v1/apps/myapp", "", http.StatusOK, nil},
	} {
		rnr, cancel := testRunner(t)
		srv := testServer(test.ds, &mqs.Mock{}, test.logDB, rnr)

		_, rec := routerRequest(t, srv.Router, "DELETE", test.path, nil)

		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
				t.Log(buf.String())
				t.Errorf("Test %d: Expected error message to have `%s`",
					i, test.expectedError.Error())
			}
		}
		cancel()
	}
}

func TestAppList(t *testing.T) {
	buf := setLogBuffer()

	rnr, cancel := testRunner(t)
	defer cancel()
	ds := datastore.NewMock()
	fnl := logs.NewMock()
	srv := testServer(ds, &mqs.Mock{}, fnl, rnr)

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{"/v1/apps", "", http.StatusOK, nil},
	} {
		_, rec := routerRequest(t, srv.Router, "GET", test.path, nil)

		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
				t.Log(buf.String())
				t.Errorf("Test %d: Expected error message to have `%s`",
					i, test.expectedError.Error())
			}
		}
	}
}

func TestAppGet(t *testing.T) {
	buf := setLogBuffer()

	rnr, cancel := testRunner(t)
	defer cancel()
	ds := datastore.NewMock()
	fnl := logs.NewMock()
	srv := testServer(ds, &mqs.Mock{}, fnl, rnr)

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{"/v1/apps/myapp", "", http.StatusNotFound, nil},
	} {
		_, rec := routerRequest(t, srv.Router, "GET", test.path, nil)

		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
				t.Log(buf.String())
				t.Errorf("Test %d: Expected error message to have `%s`",
					i, test.expectedError.Error())
			}
		}
	}
}

func TestAppUpdate(t *testing.T) {
	buf := setLogBuffer()

	for i, test := range []struct {
		mock          models.Datastore
		logDB         models.LogStore
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		// errors
		{datastore.NewMock(), logs.NewMock(), "/v1/apps/myapp", ``, http.StatusBadRequest, models.ErrInvalidJSON},

		// success
		{datastore.NewMockInit(
			[]*models.App{{
				Name: "myapp",
			}}, nil, nil,
		), logs.NewMock(), "/v1/apps/myapp", `{ "app": { "config": { "test": "1" } } }`, http.StatusOK, nil},

		// Addresses #380
		{datastore.NewMockInit(
			[]*models.App{{
				Name: "myapp",
			}}, nil, nil,
		), logs.NewMock(), "/v1/apps/myapp", `{ "app": { "name": "othername" } }`, http.StatusConflict, nil},
	} {
		rnr, cancel := testRunner(t)
		srv := testServer(test.mock, &mqs.Mock{}, test.logDB, rnr)

		body := bytes.NewBuffer([]byte(test.body))
		_, rec := routerRequest(t, srv.Router, "PATCH", test.path, body)

		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
				t.Log(buf.String())
				t.Errorf("Test %d: Expected error message to have `%s`",
					i, test.expectedError.Error())
			}
		}

		cancel()
	}
}
