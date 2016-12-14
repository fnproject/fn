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
	"github.com/iron-io/functions/api/runner/task"
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

func mockTasksConduit() chan task.Request {
	tasks := make(chan task.Request)
	go func() {
		for range tasks {
		}
	}()
	return tasks
}

func TestAppCreate(t *testing.T) {
	buf := setLogBuffer()
	tasks := mockTasksConduit()
	defer close(tasks)
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
		{&datastore.Mock{}, "/v1/apps", `{ "app": { "name": "teste" } }`, http.StatusOK, nil},
	} {
		rnr, cancel := testRunner(t)
		router := testRouter(test.mock, &mqs.Mock{}, rnr, tasks)

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
	tasks := mockTasksConduit()
	defer close(tasks)

	for i, test := range []struct {
		ds            models.Datastore
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{&datastore.Mock{}, "/v1/apps/myapp", "", http.StatusNotFound, nil},
		{&datastore.Mock{
			Apps: []*models.App{{
				Name: "myapp",
			}},
		}, "/v1/apps/myapp", "", http.StatusOK, nil},
	} {
		rnr, cancel := testRunner(t)
		router := testRouter(test.ds, &mqs.Mock{}, rnr, tasks)

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
		cancel()
	}
}

func TestAppList(t *testing.T) {
	buf := setLogBuffer()
	tasks := mockTasksConduit()
	defer close(tasks)

	rnr, cancel := testRunner(t)
	defer cancel()
	router := testRouter(&datastore.Mock{}, &mqs.Mock{}, rnr, tasks)

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
	tasks := mockTasksConduit()
	defer close(tasks)

	rnr, cancel := testRunner(t)
	defer cancel()
	router := testRouter(&datastore.Mock{}, &mqs.Mock{}, rnr, tasks)

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
	tasks := mockTasksConduit()
	defer close(tasks)

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
			Apps: []*models.App{{
				Name: "myapp",
			}},
		}, "/v1/apps/myapp", `{ "app": { "config": { "test": "1" } } }`, http.StatusOK, nil},

		// Addresses #380
		{&datastore.Mock{
			Apps: []*models.App{{
				Name: "myapp",
			}},
		}, "/v1/apps/myapp", `{ "app": { "name": "othername" } }`, http.StatusBadRequest, nil},
	} {
		rnr, cancel := testRunner(t)
		router := testRouter(test.mock, &mqs.Mock{}, rnr, tasks)

		body := bytes.NewBuffer([]byte(test.body))
		_, rec := routerRequest(t, router, "PATCH", test.path, body)

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
