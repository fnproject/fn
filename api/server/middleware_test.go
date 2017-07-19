package server

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

type middleWareStruct struct {
	name string
}

func (m *middleWareStruct) Chain(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(m.name + ","))
		next.ServeHTTP(w, r)
	})
}

func TestMiddleWareChaining(t *testing.T) {
	var lastHandler http.Handler
	lastHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("last"))
	})

	s := Server{}
	s.AddMiddleware(&middleWareStruct{"first"})
	s.AddMiddleware(&middleWareStruct{"second"})
	s.AddMiddleware(&middleWareStruct{"third"})
	s.AddMiddleware(&middleWareStruct{"fourth"})

	rec := httptest.NewRecorder()
	req, _ := http.NewRequest("get", "http://localhost/", nil)

	s.chainAndServe(rec, req, lastHandler)

	result, err := ioutil.ReadAll(rec.Result().Body)
	if err != nil {
		t.Fatal(err)
	}

	if string(result) != "first,second,third,fourth,last" {
		t.Fatal("You failed to chain correctly.", string(result))
	}
}
