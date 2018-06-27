package server

import (
	"fmt"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/url"
	"strings"
)

//TriggerAnnotator Is used to inject trigger context (such as request URLs) into outbound trigger resources
type TriggerAnnotator interface {
	// Annotates a trigger on read
	AnnotateTrigger(ctx *gin.Context, a *models.App, t *models.Trigger) (*models.Trigger, error)
}

type requestBasedTriggerAnnotator struct{}

func annotateTriggerWithBaseUrl(baseURL string, app *models.App, t *models.Trigger) (*models.Trigger, error) {
	if t.Type != models.TriggerTypeHTTP {
		return t, nil
	}

	baseURL = strings.TrimSuffix(baseURL, "/")
	src := strings.TrimPrefix(t.Source, "/")
	triggerPath := fmt.Sprintf("%s/t/%s/%s", baseURL, app.Name, src)

	newT := t.Clone()
	newAnnotations, err := newT.Annotations.With(models.TriggerHTTPEndpointAnnotation, triggerPath)
	if err != nil {
		return nil, err
	}
	newT.Annotations = newAnnotations
	return newT, nil
}

func (tp *requestBasedTriggerAnnotator) AnnotateTrigger(ctx *gin.Context, app *models.App, t *models.Trigger) (*models.Trigger, error) {

	//No, I don't feel good about myself either
	r := ctx.Value(0)
	if r == nil {
		return t, nil
	}
	rq, ok := r.(*http.Request)
	if !ok {
		return t, nil
	}

	scheme := "http"
	if rq.TLS != nil {
		scheme = "https"
	}

	return annotateTriggerWithBaseUrl(fmt.Sprintf("%s://%s", scheme, rq.Host), app, t)
}

func NewRequestBasedTriggerAnnotator() TriggerAnnotator {
	return &requestBasedTriggerAnnotator{}
}

type staticUrlTriggerAnnotator struct {
	urlBase string
}

func NewStaticURLTriggerAnnotator(baseUrl string) (TriggerAnnotator, error) {

	_, err := url.ParseRequestURI(baseUrl)
	if err != nil {
		return nil, err
	}

	return &staticUrlTriggerAnnotator{urlBase: baseUrl}, nil
}

func (s *staticUrlTriggerAnnotator) AnnotateTrigger(ctx *gin.Context, app *models.App, trigger *models.Trigger) (*models.Trigger, error) {
	return annotateTriggerWithBaseUrl(s.urlBase, app, trigger)

}
