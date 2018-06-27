package server

import (
	"context"
	"encoding/json"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
	"net/url"
	"testing"
)

func fakeGinContextWithHostTLS(host string, tls bool) context.Context {

}

func TestAnnotateTriggerDefaultProvider(t *testing.T) {
	baseUrl, err := url.Parse("https://my-server.com:8080/somePath")
	if err != nil {
		t.Fatalf("bad URL", err)
	}
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

	newT, err := tep.AnnotateTrigger(gin.Context{}, baseUrl, app, tr)

	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}

	bytes, got := newT.Annotations.Get(models.TriggerHTTPEndpointAnnotation)
	if !got {
		t.Fatalf("Expecting annotation %s to be present but go %v", newT.Annotations)
	}
	var annot string
	err = json.Unmarshal(bytes, &annot)
	if err != nil {
		t.Fatalf("Couldn't get annotation")
	}

	expected := "https://my-server.com:8080/somePath/t/myApp/url/to/somewhere"
	if annot != expected {
		t.Errorf("expected annotation to be %s but was %s", expected, annot)
	}
}

func TestNonHttpTrigger(t *testing.T) {
	tep := NewRequestBasedTriggerAnnotator()

	tr := &models.Trigger{
		Name:   "myTrigger",
		Type:   "other",
		AppID:  "",
		Source: "/url/to/somewhere",
	}

	newT, err := tep.AnnotateTrigger(context.Background(), nil, tr.Clone())

	if err != nil {
		t.Fatalf("error annotating trigger %s", err)
	}

	if !newT.Equals(tr) {
		t.Errorf("expecting non-http  trigger to be ignored")
	}

}
