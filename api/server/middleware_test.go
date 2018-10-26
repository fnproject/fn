package server

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"fmt"

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
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()

	app1 := &models.App{ID: "app_id_1", Name: "myapp", Config: models.Config{}}
	app2 := &models.App{ID: "app_id_2", Name: "myapp2", Config: models.Config{}}
	ds := datastore.NewMockInit(
		[]*models.App{app1, app2},
		[]*models.Fn{
			{ID: "fn_id1", AppID: app1.ID, Image: "fnproject/fn-test-utils"},
			{ID: "fn_id2", AppID: app1.ID, Image: "fnproject/fn-test-utils"},
			{ID: "fn_id3", AppID: app2.ID, Image: "fnproject/fn-test-utils"},
		},
	)

	rnr, cancelrnr := testRunner(t, ds)
	defer cancelrnr()

	fnl := logs.NewMock()
	srv := testServer(ds, &mqs.Mock{}, fnl, rnr, ServerTypeFull)
	srv.AddRootMiddlewareFunc(func(next http.Handler) http.Handler {
		// this one will override a call to the API based on a header
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("funcit") != "" {
				t.Log("breaker breaker!")
				w.Write([]byte("Rerooted"))
				return
			}
			// If any context changes, user should use this: next.ServeHTTP(w, r.WithContext(ctx))
			next.ServeHTTP(w, r)
		})
	})
	srv.AddRootMiddlewareFunc(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Log("middle log")
			next.ServeHTTP(w, r)
		})
	})
	srv.AddRootMiddleware(&middleWareStruct{"middle"})
	srv.AddRootMiddlewareFunc(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Log("body reader log")
			bodyBytes, _ := ioutil.ReadAll(r.Body)
			r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
			next.ServeHTTP(w, r)
		})
	})

	for i, test := range []struct {
		path           string
		body           string
		method         string
		headers        map[string][]string
		expectedCode   int
		expectedInBody string
	}{
		{"/invoke/fn_id1", `{"isDebug": true}`, "POST", map[string][]string{}, http.StatusOK, "middle"},
		{"/v2/apps/app_id_1/fns/fn_id1", `{"isDebug": true}`, "POST", map[string][]string{}, http.StatusOK, "middle"},
		{"/v2/apps", `{"isDebug": true}`, "POST", map[string][]string{"funcit": {"Test"}}, http.StatusOK, "Rerooted"},
	} {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			body := strings.NewReader(test.body)
			req, err := http.NewRequest(test.method, "http://127.0.0.1:8080"+test.path, body)
			if err != nil {
				t.Fatalf("Test: Could not create %s request to %s: %v", test.method, test.path, err)
			}
			for k, v := range test.headers {
				req.Header.Add(k, v[0])
			}
			t.Log("TESTING:", req.URL.String())
			_, rec := routerRequest2(t, srv.Router, req)
			// t.Log("REC: %+v\n", rec)

			result, err := ioutil.ReadAll(rec.Result().Body)
			if err != nil {
				t.Fatal(err)
			}

			rbody := string(result)
			t.Logf("Test %v: response body: %v", i, rbody)
			if !strings.Contains(rbody, test.expectedInBody) {
				t.Fatal(i, "middleware didn't work correctly", string(result))
			}
		})

	}

	req, err := http.NewRequest("POST", "http://127.0.0.1:8080/v2/apps", strings.NewReader("{\"name\": \"myapp3\"}"))
	if err != nil {
		t.Fatalf("Test: Could not create create app request")
	}
	t.Log("TESTING: Create myapp3 when a middleware reads the body")
	_, rec := routerRequest2(t, srv.Router, req)

	res, _ := ioutil.ReadAll(rec.Result().Body)
	if !strings.Contains(string(res), "myapp3") {
		t.Fatal("Middleware did not pass the request correctly to route handler")
	}
}
