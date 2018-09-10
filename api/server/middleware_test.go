package server

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

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
