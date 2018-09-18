package server

import (
	"fmt"
	"net/http"
	"net/textproto"
	"strconv"

	"strings"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

// handleHTTPTriggerCall executes the function, for router handlers
func (s *Server) handleHTTPTriggerCall(c *gin.Context) {
	err := s.handleTriggerHTTPFunctionCall2(c)
	if err != nil {
		handleErrorResponse(c, err)
	}
}

// handleTriggerHTTPFunctionCall2 executes the function and returns an error
// Requires the following in the context:
func (s *Server) handleTriggerHTTPFunctionCall2(c *gin.Context) error {
	ctx := c.Request.Context()
	p := c.Param(api.ParamTriggerSource)
	if p == "" {
		p = "/"
	}

	appName := c.Param(api.ParamAppName)

	appID, err := s.lbReadAccess.GetAppID(ctx, appName)
	if err != nil {
		return err
	}

	app, err := s.lbReadAccess.GetAppByID(ctx, appID)
	if err != nil {
		return err
	}

	routePath := p

	trigger, err := s.lbReadAccess.GetTriggerBySource(ctx, appID, "http", routePath)

	if err != nil {
		return err
	}

	fn, err := s.lbReadAccess.GetFnByID(ctx, trigger.FnID)
	if err != nil {
		return err
	}
	// gin sets this to 404 on NoRoute, so we'll just ensure it's 200 by default.
	c.Status(200) // this doesn't write the header yet

	return s.ServeHTTPTrigger(c, app, fn, trigger)
}

type triggerResponseWriter struct {
	inner     http.ResponseWriter
	committed bool
}

func (trw *triggerResponseWriter) Header() http.Header {
	return trw.inner.Header()
}

func (trw *triggerResponseWriter) Write(b []byte) (int, error) {
	if !trw.committed {
		trw.WriteHeader(http.StatusOK)
	}
	return trw.inner.Write(b)
}

func (trw *triggerResponseWriter) WriteHeader(statusCode int) {
	if trw.committed {
		return
	}
	trw.committed = true

	for k, vs := range trw.Header() {
		if strings.HasPrefix(k, "Fn-Http-H-") {
			// TODO strip out content-length and stuff here.
			realHeader := strings.TrimPrefix(k, "Fn-Http-H-")
			if realHeader != "" { // case where header is exactly the prefix
				for _, v := range vs {
					trw.Header().Del(k)
					trw.Header().Add(realHeader, v)
				}
			}
		}
	}

	gatewayStatus := 200

	if statusCode >= 400 {
		gatewayStatus = 502
	}

	status := trw.Header().Get("Fn-Http-Status")
	if status != "" {
		statusInt, err := strconv.Atoi(status)
		if err == nil {
			gatewayStatus = statusInt
		}
		trw.Header().Del("Fn-Http-Status")
	}

	trw.inner.WriteHeader(gatewayStatus)
}

//ServeHTTPTr	igger serves an HTTP trigger for a given app/fn/trigger  based on the current request
// This is exported to allow extensions to handle their own trigger naming and publishing
func (s *Server) ServeHTTPTrigger(c *gin.Context, app *models.App, fn *models.Fn, trigger *models.Trigger) error {
	// TODO modify all the headers here to add Fn-Http-H & method & req url

	// transpose trigger headers into HTTP
	req := c.Request
	headers := make(http.Header, len(req.Header))
	for k, vs := range req.Header {
		// should be generally unnecessary but to be doubly sure.
		k = textproto.CanonicalMIMEHeaderKey(k)
		if skipTriggerHeaders[k] {
			continue
		}
		rewriteKey := fmt.Sprintf("Fn-Http-H-%s", k)
		for _, v := range vs {
			headers.Add(rewriteKey, v)
		}
	}
	requestUrl := reqURL(req)

	headers.Set("Fn-Http-Method", req.Method)
	headers.Set("Fn-Http-Request-Url", requestUrl)
	headers.Set("Fn-Intent", "httprequest")
	req.Header = headers

	c.Writer = &triggerResponseWriter{inner: c.Writer}

	return s.fnInvoke(c, app, fn, trigger)
}

var skipTriggerHeaders = map[string]bool{
	"Connection":        true,
	"Keep-Alive":        true,
	"Trailer":           true,
	"Transfer-Encoding": true,
	"TE":                true,
	"Upgrade":           true,
}

func reqURL(req *http.Request) string {
	if req.URL.Scheme == "" {
		if req.TLS == nil {
			req.URL.Scheme = "http"
		} else {
			req.URL.Scheme = "https"
		}
	}
	if req.URL.Host == "" {
		req.URL.Host = req.Host
	}
	return req.URL.String()
}
