package server

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// handleFnInvokeCall executes the function, for router handlers
func (s *Server) handleFnInvokeCall(c *gin.Context) {
	//ap := c.Param(api.AppID)
	fnID := c.Param(api.ParamFnID)
	ctx, _ := common.LoggerWithFields(c.Request.Context(), logrus.Fields{"fnID": fnID})
	c.Request = c.Request.WithContext(ctx)
	err := s.handleFnInvokeCall2(c)
	if err != nil {
		handleErrorResponse(c, err)
	}
}

// handleTriggerHTTPFunctionCall2 executes the function and returns an error
// Requires the following in the context:
func (s *Server) handleFnInvokeCall2(c *gin.Context) error {
	//	log := common.Logger(c.Request.Context())

	fn, err := s.lbReadAccess.GetFnByID(c, c.Param(api.ParamFnID))
	if err != nil {
		return err
	}

	app, err := s.lbReadAccess.GetAppByID(c, fn.AppID)
	if err != nil {
		return err
	}

	return s.ServeFnInvoke(c, app, fn)
}

func (s *Server) ServeFnInvoke(c *gin.Context, app *models.App, fn *models.Fn) error {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	writer := syncResponseWriter{
		Buffer:  buf,
		headers: c.Writer.Header(),
	}
	defer bufPool.Put(buf) // TODO need to ensure this is safe with Dispatch?

	call, err := s.agent.GetCall(
		agent.WithWriter(&writer), // XXX (reed): order matters [for now]
		agent.FromHTTPFnRequest(app, fn, c.Request),
	)
	if err != nil {
		return err
	}

	model := call.Model()
	{ // scope this, to disallow ctx use outside of this scope. add id for handleV1ErrorResponse logger
		ctx, _ := common.LoggerWithFields(c.Request.Context(), logrus.Fields{"id": model.ID})
		c.Request = c.Request.WithContext(ctx)
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
	io.Copy(c.Writer, &writer)

	return nil
}
