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

	var fnStatus int
	realHeaders := trw.Header()
	gwHeaders := make(http.Header, len(realHeaders))
	for k, vs := range realHeaders {
		switch {
		case strings.HasPrefix(k, "Fn-Http-H-"):
			gwHeader := strings.TrimPrefix(k, "Fn-Http-H-")
			if gwHeader != "" { // case where header is exactly the prefix
				gwHeaders[gwHeader] = vs
			}
		case k == "Fn-Http-Status":
			if len(vs) > 0 {
				statusInt, err := strconv.Atoi(vs[0])
				if err == nil {
					fnStatus = statusInt
				}
			}
		case k == "Content-Type", k == "Fn-Call-Id":
			gwHeaders[k] = vs
		}
	}

	// XXX(reed): this is O(3n)... yes sorry for making it work without making it perfect first
	for k := range realHeaders {
		realHeaders.Del(k)
	}
	for k, vs := range gwHeaders {
		realHeaders[k] = vs
	}

	// XXX(reed): simplify / add tests for these behaviors...
	gatewayStatus := 200
	if statusCode >= 400 {
		gatewayStatus = 502
	} else if fnStatus > 0 {
		gatewayStatus = fnStatus
	}

	trw.inner.WriteHeader(gatewayStatus)
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

// ServeHTTPTrigger serves an HTTP trigger for a given app/fn/trigger based on the current request
// This is exported to allow extensions to handle their own trigger naming and publishing
func (s *Server) ServeHTTPTrigger(c *gin.Context, app *models.App, fn *models.Fn, trigger *models.Trigger) error {
	// transpose trigger headers into the request
	req := c.Request
	headers := make(http.Header, len(req.Header))
	for k, vs := range req.Header {
		// should be generally unnecessary but to be doubly sure.
		k = textproto.CanonicalMIMEHeaderKey(k)
		if skipTriggerHeaders[k] {
			continue
		}
		switch k {
		case "Content-Type":
		default:
			k = fmt.Sprintf("Fn-Http-H-%s", k)
		}
		headers[k] = vs
	}
	requestURL := reqURL(req)

	headers.Set("Fn-Http-Method", req.Method)
	headers.Set("Fn-Http-Request-Url", requestURL)
	headers.Set("Fn-Intent", "httprequest")
	req.Header = headers

	// trap the headers and rewrite them for http trigger
	rw := &triggerResponseWriter{inner: c.Writer}

	return s.fnInvoke(rw, req, app, fn, trigger)
}
