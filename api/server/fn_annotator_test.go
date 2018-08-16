package server

import (
	bytes2 "bytes"
	"crypto/tls"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func TestAnnotateFnDefaultProvider(t *testing.T) {

	app := &models.App{
		ID:   "app_id",
		Name: "myApp",
	}

	tr := &models.Fn{
		ID:    "fnID",
		Name:  "myFn",
		AppID: app.ID,
	}

	// defaults the fn endpoint to the base URL if it's not already set
	tep := NewRequestBasedFnAnnotator()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/v2/foo/bar", bytes2.NewBuffer([]byte{}))
	c.Request.Host = "my-server.com:8192"
	newT, err := tep.AnnotateFn(c, app, tr)

	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}

	bytes, got := newT.Annotations.Get(models.FnInvokeEndpointAnnotation)
	if !got {
		t.Fatalf("Expecting annotation to be present but got %v", newT.Annotations)
	}

	var annot string
	err = json.Unmarshal(bytes, &annot)
	if err != nil {
		t.Fatalf("Couldn't get annotation")
	}

	expected := "http://my-server.com:8192/invoke/fnID"
	if annot != expected {
		t.Errorf("expected annotation to be %s but was %s", expected, annot)
	}
}

func TestHttpsFn(t *testing.T) {

	app := &models.App{
		ID:   "app_id",
		Name: "myApp",
	}

	tr := &models.Fn{
		ID:    "fnID",
		Name:  "myFn",
		AppID: app.ID,
	}

	// defaults the Fn endpoint to the base URL if it's not already set
	tep := NewRequestBasedFnAnnotator()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/v2/foo/bar", bytes2.NewBuffer([]byte{}))
	c.Request.Host = "my-server.com:8192"
	c.Request.TLS = &tls.ConnectionState{}

	newT, err := tep.AnnotateFn(c, app, tr)

	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}

	bytes, got := newT.Annotations.Get(models.FnInvokeEndpointAnnotation)
	if !got {
		t.Fatalf("Expecting annotation  to be present but got %v", newT.Annotations)
	}
	var annot string
	err = json.Unmarshal(bytes, &annot)
	if err != nil {
		t.Fatalf("Couldn't get annotation")
	}

	expected := "https://my-server.com:8192/invoke/fnID"
	if annot != expected {
		t.Errorf("expected annotation to be %s but was %s", expected, annot)
	}
}

func TestStaticUrlFnAnnotator(t *testing.T) {
	a := NewStaticURLFnAnnotator("http://foo.bar.com/somewhere")

	app := &models.App{
		ID:   "app_id",
		Name: "myApp",
	}

	tr := &models.Fn{
		ID:    "fnID",
		Name:  "myFn",
		AppID: app.ID,
	}

	newT, err := a.AnnotateFn(nil, app, tr)
	if err != nil {
		t.Fatalf("failed when should have succeeded: %s", err)
	}

	bytes, got := newT.Annotations.Get(models.FnInvokeEndpointAnnotation)
	if !got {
		t.Fatalf("Expecting annotation to be present but got %v", newT.Annotations)
	}
	var annot string
	err = json.Unmarshal(bytes, &annot)
	if err != nil {
		t.Fatalf("Couldn't get annotation")
	}

	expected := "http://foo.bar.com/somewhere/invoke/fnID"
	if annot != expected {
		t.Errorf("expected annotation to be %s but was %s", expected, annot)
	}

}
