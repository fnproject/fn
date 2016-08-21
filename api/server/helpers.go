package server

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/net/context"

	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/runner"
	titancommon "github.com/iron-io/titan/common"
)

type appResponse struct {
	Message string
	App     models.AppWrapper
}

type appsResponse struct {
	Message string
	Apps    models.AppsWrapper
}

type routeResponse struct {
	Message string
	Route   models.RouteWrapper
}

type routesResponse struct {
	Message string
	Routes  models.RoutesWrapper
}

func testRouter() *gin.Engine {
	r := gin.Default()
	ctx := context.Background()
	r.Use(func(c *gin.Context) {
		ctx, _ := titancommon.LoggerWithFields(ctx, extractFields(c))
		c.Set("ctx", ctx)
		c.Next()
	})
	bindHandlers(r)
	return r
}

func testRunner(t *testing.T) *runner.Runner {
	r, err := runner.New()
	if err != nil {
		t.Fatal("Test: failed to create new runner")
	}
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
