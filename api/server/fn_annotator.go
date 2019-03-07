package server

import (
	"fmt"
	"strings"

	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

//FnAnnotator Is used to inject fn context (such as request URLs) into outbound fn resources
type FnAnnotator interface {
	// Annotates a trigger on read
	AnnotateFn(ctx *gin.Context, a *models.App, fn *models.Fn) (*models.Fn, error)
}

type requestBasedFnAnnotator struct {
	group, template string
}

func annotateFnWithBaseURL(baseURL, group, template string, app *models.App, fn *models.Fn) (*models.Fn, error) {

	baseURL = strings.TrimSuffix(baseURL, "/")
	path := strings.Replace(template, ":fn_id", fn.ID, -1)
	invokePath := fmt.Sprintf("%s%s%s", baseURL, group, path)

	newT := fn.Clone()
	newAnnotations, err := newT.Annotations.With(models.FnInvokeEndpointAnnotation, invokePath)
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

	return annotateFnWithBaseURL(fmt.Sprintf("%s://%s", scheme, ctx.Request.Host), tp.group, tp.template, app, t)
}

//NewRequestBasedFnAnnotator creates a FnAnnotator that inspects the incoming request host and port, and uses this to generate fn invoke endpoint URLs based on those
func NewRequestBasedFnAnnotator(group, template string) FnAnnotator {
	return &requestBasedFnAnnotator{group: group, template: template}
}

type staticURLFnAnnotator struct {
	baseURL, group, template string
}

//NewStaticURLFnAnnotator annotates triggers bases on a given, specified URL base - e.g. "https://my.domain" --->  "https://my.domain/t/app/source"
func NewStaticURLFnAnnotator(baseURL, group, template string) FnAnnotator {

	return &staticURLFnAnnotator{baseURL: baseURL, group: group, template: template}
}

func (s *staticURLFnAnnotator) AnnotateFn(ctx *gin.Context, app *models.App, trigger *models.Fn) (*models.Fn, error) {
	return annotateFnWithBaseURL(s.baseURL, s.group, s.template, app, trigger)

}
