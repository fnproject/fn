package server

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
	"github.com/fnproject/fn/fnext"
	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	os.Exit(m.Run())
}

type middleWareStruct struct {
	name string
}

func (m *middleWareStruct) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(m.name + ","))
		next.ServeHTTP(w, r)
	})
}

func TestMiddlewareChaining(t *testing.T) {
	var lastHandler http.Handler
	lastHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("last"))
	})

	s := Server{}
	s.AddAPIMiddleware(&middleWareStruct{"first"})
	s.AddAPIMiddleware(&middleWareStruct{"second"})
	s.AddAPIMiddleware(&middleWareStruct{"third"})
	s.AddAPIMiddleware(&middleWareStruct{"fourth"})
	c := &gin.Context{}

	rec := httptest.NewRecorder()
	req, _ := http.NewRequest("get", "http://localhost/", nil)
	ctx := context.WithValue(req.Context(), fnext.MiddlewareControllerKey, s.newMiddlewareController(c))
	req = req.WithContext(ctx)
	c.Request = req

	chainAndServe(s.apiMiddlewares, rec, req, lastHandler)

	result, err := ioutil.ReadAll(rec.Result().Body)
	if err != nil {
		t.Fatal(err)
	}

	if string(result) != "first,second,third,fourth,last" {
		t.Fatal("You failed to chain correctly:", string(result))
	}
}

func TestRootMiddleware(t *testing.T) {

	app1 := &models.App{Name: "myapp", Config: models.Config{}}
	app1.SetDefaults()
	app2 := &models.App{Name: "myapp2", Config: models.Config{}}
	app2.SetDefaults()
	ds := datastore.NewMockInit(
		[]*models.App{app1, app2},
		[]*models.Route{
			{Path: "/", AppID: app1.ID, AppName: "myapp", Image: "fnproject/fn-test-utils", Type: "sync", Memory: 128, CPUs: 100, Timeout: 30, IdleTimeout: 30, Headers: map[string][]string{"X-Function": {"Test"}}},
			{Path: "/myroute", AppID: app1.ID, AppName: "myapp", Image: "fnproject/fn-test-utils", Type: "sync", Memory: 128, Timeout: 30, IdleTimeout: 30, Headers: map[string][]string{"X-Function": {"Test"}}},
			{Path: "/app2func", AppID: app2.ID, AppName: "myapp2", Image: "fnproject/fn-test-utils", Type: "sync", Memory: 128, Timeout: 30, IdleTimeout: 30, Headers: map[string][]string{"X-Function": {"Test"}},
				Config: map[string]string{"NAME": "johnny"},
			},
		}, nil,
	)

	rnr, cancelrnr := testRunner(t, ds)
	defer cancelrnr()

	fnl := logs.NewMock()
	srv := testServer(ds, &mqs.Mock{}, fnl, rnr, ServerTypeFull)
	srv.AddRootMiddlewareFunc(func(next http.Handler) http.Handler {
		// this one will override a call to the API based on a header
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("funcit") != "" {
				fmt.Fprintf(os.Stderr, "breaker breaker!\n")
				ctx := r.Context()
				// TODO: this is a little dicey, should have some functions to set these in case the context keys change or something.
				ctx = context.WithValue(ctx, "app", "myapp2")
				ctx = context.WithValue(ctx, "path", "/app2func")
				mctx := fnext.GetMiddlewareController(ctx)
				mctx.CallFunction(w, r.WithContext(ctx))
				return
			}
			// If any context changes, user should use this: next.ServeHTTP(w, r.WithContext(ctx))
			next.ServeHTTP(w, r)
		})
	})
	srv.AddRootMiddlewareFunc(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(os.Stderr, "middle log\n")
			next.ServeHTTP(w, r)
		})
	})
	srv.AddRootMiddleware(&middleWareStruct{"middle"})

	for i, test := range []struct {
		path           string
		body           string
		method         string
		headers        map[string][]string
		expectedCode   int
		expectedInBody string
	}{
		{"/r/myapp", `{"isDebug": true}`, "GET", map[string][]string{}, http.StatusOK, "middle"},
		{"/r/myapp/myroute", `{"isDebug": true}`, "GET", map[string][]string{}, http.StatusOK, "middle"},
		{"/v1/apps", `{"isDebug": true}`, "GET", map[string][]string{"funcit": {"Test"}}, http.StatusOK, "johnny"},
	} {
		body := strings.NewReader(test.body)
		req, err := http.NewRequest(test.method, "http://127.0.0.1:8080"+test.path, body)
		if err != nil {
			t.Fatalf("Test: Could not create %s request to %s: %v", test.method, test.path, err)
		}
		for k, v := range test.headers {
			req.Header.Add(k, v[0])
		}
		fmt.Println("TESTING:", req.URL.String())
		_, rec := routerRequest2(t, srv.Router, req)
		// t.Log("REC: %+v\n", rec)

		result, err := ioutil.ReadAll(rec.Result().Body)
		if err != nil {
			t.Fatal(err)
		}

		rbody := string(result)
		t.Log("rbody:", rbody)
		if !strings.Contains(rbody, test.expectedInBody) {
			t.Fatal(i, "middleware didn't work correctly", string(result))
		}
	}
}
