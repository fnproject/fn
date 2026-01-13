package server

import (
	"fmt"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
	"strings"
)

// TriggerAnnotator Is used to inject trigger context (such as request URLs) into outbound trigger resources
type TriggerAnnotator interface {
	// Annotates a trigger on read
	AnnotateTrigger(ctx *gin.Context, a *models.App, t *models.Trigger) (*models.Trigger, error)
}

type requestBasedTriggerAnnotator struct{}

func annotateTriggerWithBaseURL(baseURL string, app *models.App, t *models.Trigger) (*models.Trigger, error) {
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
	scheme := "http"
	if ctx.Request.TLS != nil {
		scheme = "https"
	}

	return annotateTriggerWithBaseURL(fmt.Sprintf("%s://%s", scheme, ctx.Request.Host), app, t)
}

// NewRequestBasedTriggerAnnotator creates a TriggerAnnotator that inspects the incoming request host and port, and uses this to generate http trigger endpoint URLs based on those
func NewRequestBasedTriggerAnnotator() TriggerAnnotator {
	return &requestBasedTriggerAnnotator{}
}

type staticURLTriggerAnnotator struct {
	baseURL string
}

// NewStaticURLTriggerAnnotator annotates triggers bases on a given, specified URL base - e.g. "https://my.domain" --->  "https://my.domain/t/app/source"
func NewStaticURLTriggerAnnotator(baseURL string) TriggerAnnotator {

	return &staticURLTriggerAnnotator{baseURL: baseURL}
}

func (s *staticURLTriggerAnnotator) AnnotateTrigger(ctx *gin.Context, app *models.App, trigger *models.Trigger) (*models.Trigger, error) {
	return annotateTriggerWithBaseURL(s.baseURL, app, trigger)

}
