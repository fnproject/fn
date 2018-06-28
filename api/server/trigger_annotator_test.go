package server

import (
	bytes2 "bytes"
	"crypto/tls"
	"encoding/json"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestAnnotateTriggerDefaultProvider(t *testing.T) {

	app := &models.App{
		ID:   "app_id",
		Name: "myApp",
	}

	tr := &models.Trigger{
		Name:   "myTrigger",
		Type:   "http",
		AppID:  app.ID,
		Source: "/url/to/somewhere",
	}

	// defaults the trigger endpoint to the base URL if it's not already set
	tep := NewRequestBasedTriggerAnnotator()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/v2/foo/bar", bytes2.NewBuffer([]byte{}))
	c.Request.Host = "my-server.com:8192"
	newT, err := tep.AnnotateTrigger(c, app, tr)

	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}

	bytes, got := newT.Annotations.Get(models.TriggerHTTPEndpointAnnotation)
	if !got {
		t.Fatalf("Expecting annotation to be present but got %v", newT.Annotations)
	}

	var annot string
	err = json.Unmarshal(bytes, &annot)
	if err != nil {
		t.Fatalf("Couldn't get annotation")
	}

	expected := "http://my-server.com:8192/t/myApp/url/to/somewhere"
	if annot != expected {
		t.Errorf("expected annotation to be %s but was %s", expected, annot)
	}
}

func TestNonHttpTrigger(t *testing.T) {
	tep := NewRequestBasedTriggerAnnotator()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "http://foo.com", bytes2.NewBuffer([]byte{}))

	tr := &models.Trigger{
		Name:   "myTrigger",
		Type:   "other",
		AppID:  "",
		Source: "/url/to/somewhere",
	}

	newT, err := tep.AnnotateTrigger(c, nil, tr.Clone())

	if err != nil {
		t.Fatalf("error annotating trigger %s", err)
	}

	if !newT.Equals(tr) {
		t.Errorf("expecting non-http  trigger to be ignored")
	}

}

func TestHttpsTrigger(t *testing.T) {

	app := &models.App{
		ID:   "app_id",
		Name: "myApp",
	}

	tr := &models.Trigger{
		Name:   "myTrigger",
		Type:   "http",
		AppID:  app.ID,
		Source: "/url/to/somewhere",
	}

	// defaults the trigger endpoint to the base URL if it's not already set
	tep := NewRequestBasedTriggerAnnotator()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/v2/foo/bar", bytes2.NewBuffer([]byte{}))
	c.Request.Host = "my-server.com:8192"
	c.Request.TLS = &tls.ConnectionState{}

	newT, err := tep.AnnotateTrigger(c, app, tr)

	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}

	bytes, got := newT.Annotations.Get(models.TriggerHTTPEndpointAnnotation)
	if !got {
		t.Fatalf("Expecting annotation  to be present but got %v", newT.Annotations)
	}
	var annot string
	err = json.Unmarshal(bytes, &annot)
	if err != nil {
		t.Fatalf("Couldn't get annotation")
	}

	expected := "https://my-server.com:8192/t/myApp/url/to/somewhere"
	if annot != expected {
		t.Errorf("expected annotation to be %s but was %s", expected, annot)
	}
}

func TestStaticUrlTriggerAnnotator(t *testing.T) {
	a := NewStaticURLTriggerAnnotator("http://foo.bar.com/somewhere")

	app := &models.App{
		ID:   "app_id",
		Name: "myApp",
	}

	tr := &models.Trigger{
		Name:   "myTrigger",
		Type:   "http",
		AppID:  app.ID,
		Source: "/url/to/somewhere",
	}

	newT, err := a.AnnotateTrigger(nil, app, tr)
	if err != nil {
		t.Fatalf("failed when should hae succeeded: %s", err)
	}

	bytes, got := newT.Annotations.Get(models.TriggerHTTPEndpointAnnotation)
	if !got {
		t.Fatalf("Expecting annotation to be present but got %v", newT.Annotations)
	}
	var annot string
	err = json.Unmarshal(bytes, &annot)
	if err != nil {
		t.Fatalf("Couldn't get annotation")
	}

	expected := "http://foo.bar.com/somewhere/t/myApp/url/to/somewhere"
	if annot != expected {
		t.Errorf("expected annotation to be %s but was %s", expected, annot)
	}

}
