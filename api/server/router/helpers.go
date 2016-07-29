package router

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/server/datastore"
)

func testRouter() *gin.Engine {
	r := gin.Default()
	r.Use(func(c *gin.Context) {
		c.Set("store", &datastore.Mock{})
		c.Set("log", logrus.WithFields(logrus.Fields{}))
		c.Next()
	})
	Start(r)
	return r
}

func routerRequest(t *testing.T, router *gin.Engine, method, path string, body io.Reader) (*http.Request, *httptest.ResponseRecorder) {
	req, err := http.NewRequest(method, "http://localhost:8080"+path, body)
	if err != nil {
		t.Fatalf("Test: Could not create %s request to %s: %v", method, path, err)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

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
