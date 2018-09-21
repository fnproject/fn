package server

import (
	"bytes"
	"net/http"
	"strconv"

	"strings"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/agent"
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
	syncResponseWriter
	committed bool
}

var _ ResponseBufferingWriter = new(triggerResponseWriter)

func (trw *triggerResponseWriter) Header() http.Header {
	return trw.Headers
}

func (trw *triggerResponseWriter) Write(b []byte) (int, error) {
	if !trw.committed {
		trw.WriteHeader(http.StatusOK)
	}
	return trw.GetBuffer().Write(b)
}

func (trw *triggerResponseWriter) WriteHeader(statusCode int) {
	if trw.committed {
		return
	}
	trw.committed = true
	gatewayStatus := 200

	if statusCode >= 400 {
		gatewayStatus = 502
	}

	status := trw.Headers.Get("Fn-Http-Status")
	if status != "" {
		statusInt, err := strconv.Atoi(status)
		if err == nil {
			gatewayStatus = statusInt
		}
	}

	for k, vs := range trw.Headers {
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

	contentType := trw.Headers.Get("Content-Type")
	if contentType != "" {
		trw.Header().Add("Content-Type", contentType)
	}
	trw.WriteHeader(gatewayStatus)
}

//ServeHTTPTr	igger serves an HTTP trigger for a given app/fn/trigger  based on the current request
// This is exported to allow extensions to handle their own trigger naming and publishing
func (s *Server) ServeHTTPTrigger(c *gin.Context, app *models.App, fn *models.Fn, trigger *models.Trigger) error {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf) // TODO need to ensure this is safe with Dispatch?

	triggerWriter := &triggerResponseWriter{
		syncResponseWriter{
			Buffer:  buf,
			Headers: c.Writer.Header()},
		false,
	}
	// GetCall can mod headers, assign an id, look up the route/app (cached),
	// strip params, etc.
	// this should happen ASAP to turn app name to app ID

	// GetCall can mod headers, assign an id, look up the route/app (cached),
	// strip params, etc.
	return s.FnInvoke(c, app, fn, triggerWriter,
		agent.WithWriter(triggerWriter), // XXX (reed): order matters [for now]
		agent.FromHTTPTriggerRequest(app, fn, trigger, c.Request),
	)
}
