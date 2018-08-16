package server

import (
	"fmt"
	"strings"

	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

//FnAnnotator Is used to inject trigger context (such as request URLs) into outbound trigger resources
type FnAnnotator interface {
	// Annotates a trigger on read
	AnnotateFn(ctx *gin.Context, a *models.App, fn *models.Fn) (*models.Fn, error)
}

type requestBasedFnAnnotator struct{}

func annotateFnWithBaseURL(baseURL string, app *models.App, fn *models.Fn) (*models.Fn, error) {

	baseURL = strings.TrimSuffix(baseURL, "/")
	src := strings.TrimPrefix(fn.ID, "/")
	triggerPath := fmt.Sprintf("%s/invoke/%s", baseURL, src)

	newT := fn.Clone()
	newAnnotations, err := newT.Annotations.With(models.FnInvokeEndpointAnnotation, triggerPath)
	if err != nil {
		return nil, err
	}
	newT.Annotations = newAnnotations
	return newT, nil
}

func (tp *requestBasedFnAnnotator) AnnotateFn(ctx *gin.Context, app *models.App, t *models.Fn) (*models.Fn, error) {

	//No, I don't feel good about myself either
	scheme := "http"
	if ctx.Request.TLS != nil {
		scheme = "https"
	}

	return annotateFnWithBaseURL(fmt.Sprintf("%s://%s", scheme, ctx.Request.Host), app, t)
}

//NewRequestBasedFnAnnotator creates a FnAnnotator that inspects the incoming request host and port, and uses this to generate fn invoke endpoint URLs based on those
func NewRequestBasedFnAnnotator() FnAnnotator {
	return &requestBasedFnAnnotator{}
}

type staticURLFnAnnotator struct {
	baseURL string
}

//NewStaticURLFnAnnotator annotates triggers bases on a given, specified URL base - e.g. "https://my.domain" --->  "https://my.domain/t/app/source"
func NewStaticURLFnAnnotator(baseURL string) FnAnnotator {

	return &staticURLFnAnnotator{baseURL: baseURL}
}

func (s *staticURLFnAnnotator) AnnotateFn(ctx *gin.Context, app *models.App, trigger *models.Fn) (*models.Fn, error) {
	return annotateFnWithBaseURL(s.baseURL, app, trigger)

}
