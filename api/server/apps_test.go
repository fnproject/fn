package server

import (
	"bytes"
	"log"
	"net/http"
	"strings"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/datastore"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/mqs"
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
		mock          *datastore.Mock
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		// errors
		{&datastore.Mock{}, "/v1/apps", ``, http.StatusBadRequest, models.ErrInvalidJSON},
		{&datastore.Mock{}, "/v1/apps", `{}`, http.StatusBadRequest, models.ErrAppsMissingNew},
		{&datastore.Mock{}, "/v1/apps", `{ "name": "Test" }`, http.StatusBadRequest, models.ErrAppsMissingNew},
		{&datastore.Mock{}, "/v1/apps", `{ "app": { "name": "" } }`, http.StatusInternalServerError, models.ErrAppsValidationMissingName},
		{&datastore.Mock{}, "/v1/apps", `{ "app": { "name": "1234567890123456789012345678901" } }`, http.StatusInternalServerError, models.ErrAppsValidationTooLongName},
		{&datastore.Mock{}, "/v1/apps", `{ "app": { "name": "&&%@!#$#@$" } }`, http.StatusInternalServerError, models.ErrAppsValidationInvalidName},
		{&datastore.Mock{}, "/v1/apps", `{ "app": { "name": "&&%@!#$#@$" } }`, http.StatusInternalServerError, models.ErrAppsValidationInvalidName},

		// success
		{&datastore.Mock{}, "/v1/apps", `{ "app": { "name": "teste" } }`, http.StatusCreated, nil},
	} {
		s := New(test.mock, &mqs.Mock{}, testRunner(t))
		router := testRouter(s)

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
	}
}

func TestAppDelete(t *testing.T) {
	buf := setLogBuffer()
	s := New(&datastore.Mock{}, &mqs.Mock{}, testRunner(t))
	router := testRouter(s)

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{"/v1/apps", "", http.StatusNotFound, nil},
		{"/v1/apps/myapp", "", http.StatusOK, nil},
	} {
		_, rec := routerRequest(t, router, "DELETE", test.path, nil)

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

func TestAppList(t *testing.T) {
	buf := setLogBuffer()
	s := New(&datastore.Mock{}, &mqs.Mock{}, testRunner(t))
	router := testRouter(s)

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{"/v1/apps", "", http.StatusOK, nil},
	} {
		_, rec := routerRequest(t, router, "GET", test.path, nil)

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
	s := New(&datastore.Mock{}, &mqs.Mock{}, testRunner(t))
	router := testRouter(s)

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{"/v1/apps/myapp", "", http.StatusNotFound, nil},
	} {
		_, rec := routerRequest(t, router, "GET", test.path, nil)

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
		mock          *datastore.Mock
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		// errors
		{&datastore.Mock{}, "/v1/apps/myapp", ``, http.StatusBadRequest, models.ErrInvalidJSON},

		// success
		{&datastore.Mock{
			FakeApp: &models.App{
				Name: "myapp",
			},
		}, "/v1/apps/myapp", `{ "app": { "config": { "test": "1" } } }`, http.StatusOK, nil},
	} {
		s := New(test.mock, &mqs.Mock{}, testRunner(t))
		router := testRouter(s)

		body := bytes.NewBuffer([]byte(test.body))
		_, rec := routerRequest(t, router, "PUT", test.path, body)

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
