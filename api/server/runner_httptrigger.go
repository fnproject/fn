package server

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"time"

	"strings"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
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
	w         http.ResponseWriter
	headers   http.Header
	committed bool
}

var _ http.ResponseWriter = new(triggerResponseWriter)

func (trw *triggerResponseWriter) Header() http.Header {
	return trw.headers
}

func (trw *triggerResponseWriter) Write(b []byte) (int, error) {
	if !trw.committed {
		trw.WriteHeader(http.StatusOK)
	}
	return trw.w.Write(b)
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

	status := trw.headers.Get("Fn-Http-Status")
	if status != "" {
		statusInt, err := strconv.Atoi(status)
		if err == nil {
			gatewayStatus = statusInt
		}
	}

	for k, vs := range trw.headers {
		if strings.HasPrefix(k, "Fn-Http-H-") {
			// TODO strip out content-length and stuff here.
			realHeader := strings.TrimPrefix(k, "Fn-Http-H-")
			if realHeader != "" { // case where header is exactly the prefix
				for _, v := range vs {
					trw.w.Header().Add(realHeader, v)
				}
			}
		}
	}

	contentType := trw.headers.Get("Content-Type")
	if contentType != "" {
		trw.w.Header().Add("Content-Type", contentType)
	}
	trw.w.WriteHeader(gatewayStatus)
}

//ServeHTTPTr	igger serves an HTTP trigger for a given app/fn/trigger  based on the current request
// This is exported to allow extensions to handle their own trigger naming and publishing
func (s *Server) ServeHTTPTrigger(c *gin.Context, app *models.App, fn *models.Fn, trigger *models.Trigger) error {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	writer := &syncResponseWriter{
		Buffer:  buf,
		headers: c.Writer.Header(), // copy ref
	}
	defer bufPool.Put(buf) // TODO need to ensure this is safe with Dispatch?

	triggerWriter := &triggerResponseWriter{
		w:       writer,
		headers: make(http.Header),
	}
	// GetCall can mod headers, assign an id, look up the route/app (cached),
	// strip params, etc.
	// this should happen ASAP to turn app name to app ID

	// GetCall can mod headers, assign an id, look up the route/app (cached),
	// strip params, etc.
	call, err := s.agent.GetCall(
		agent.WithWriter(triggerWriter), // XXX (reed): order matters [for now]
		agent.FromHTTPTriggerRequest(app, fn, trigger, c.Request),
	)

	if err != nil {
		return err
	}
	model := call.Model()
	{ // scope this, to disallow ctx use outside of this scope. add id for handleV1ErrorResponse logger
		ctx, _ := common.LoggerWithFields(c.Request.Context(), logrus.Fields{"id": model.ID})
		c.Request = c.Request.WithContext(ctx)
	}
	writer.Header().Add("Fn_call_id", model.ID)

	// TODO TRIGGERWIP  not clear this makes sense here - but it works  so...
	if model.Type == "async" {

		// TODO we should push this into GetCall somehow (CallOpt maybe) or maybe agent.Queue(Call) ?
		if c.Request.ContentLength > 0 {
			buf.Grow(int(c.Request.ContentLength))
		}
		_, err := buf.ReadFrom(c.Request.Body)
		if err != nil {
			return models.ErrInvalidPayload
		}
		model.Payload = buf.String()

		err = s.lbEnqueue.Enqueue(c.Request.Context(), model)
		if err != nil {
			return err
		}

		c.JSON(http.StatusAccepted, map[string]string{"call_id": model.ID})
		return nil
	}

	err = s.agent.Submit(call)
	if err != nil {
		// NOTE if they cancel the request then it will stop the call (kind of cool),
		// we could filter that error out here too as right now it yells a little
		if err == models.ErrCallTimeoutServerBusy || err == models.ErrCallTimeout {
			// TODO maneuver
			// add this, since it means that start may not have been called [and it's relevant]
			c.Writer.Header().Add("XXX-FXLB-WAIT", time.Now().Sub(time.Time(model.CreatedAt)).String())
		}
		return err
	}

	// if they don't set a content-type - detect it
	if writer.Header().Get("Content-Type") == "" {
		// see http.DetectContentType, the go server is supposed to do this for us but doesn't appear to?
		var contentType string
		jsonPrefix := [1]byte{'{'} // stack allocated
		if bytes.HasPrefix(buf.Bytes(), jsonPrefix[:]) {
			// try to detect json, since DetectContentType isn't a hipster.
			contentType = "application/json; charset=utf-8"
		} else {
			contentType = http.DetectContentType(buf.Bytes())
		}
		writer.Header().Set("Content-Type", contentType)
	}

	writer.Header().Set("Content-Length", strconv.Itoa(int(buf.Len())))

	if writer.status > 0 {
		c.Writer.WriteHeader(writer.status)
	}
	io.Copy(c.Writer, writer)

	return nil
}
