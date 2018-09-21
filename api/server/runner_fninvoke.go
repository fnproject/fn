package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

var (
	bufPool = &sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}
)

type ResponseBufferingWriter interface {
	http.ResponseWriter
	io.Reader
	Status() int
	GetBuffer() *bytes.Buffer
}

var _ ResponseBufferingWriter = new(syncResponseWriter)

// implements http.ResponseWriter
// this little guy buffers responses from user containers and lets them still
// set headers and such without us risking writing partial output [as much, the
// server could still die while we're copying the buffer]. this lets us set
// content length and content type nicely, as a bonus. it is sad, yes.
type syncResponseWriter struct {
	Headers http.Header
	status  int
	*bytes.Buffer
}

func (s *syncResponseWriter) Header() http.Header      { return s.Headers }
func (s *syncResponseWriter) WriteHeader(code int)     { s.status = code }
func (s *syncResponseWriter) Status() int              { return s.status }
func (s *syncResponseWriter) GetBuffer() *bytes.Buffer { return s.Buffer }

// handleFnInvokeCall executes the function, for router handlers
func (s *Server) handleFnInvokeCall(c *gin.Context) {
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
	// TODO: we should get rid of the buffers, and stream back (saves memory (+splice), faster (splice), allows streaming, don't have to cap resp size)
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf) // TODO need to ensure this is safe with Dispatch?

	writer := &syncResponseWriter{
		Buffer:  buf,
		Headers: c.Writer.Header(),
	}

	return s.FnInvoke(c, app, fn, writer,
		agent.WithWriter(writer), // XXX (reed): order matters [for now]
		agent.FromHTTPFnRequest(app, fn, c.Request),
	)
}

func (s *Server) FnInvoke(c *gin.Context, app *models.App, fn *models.Fn, writer ResponseBufferingWriter, opts ...agent.CallOpt) error {
	call, err := s.agent.GetCall(
		opts...,
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
	fmt.Println(writer.Header())
	// if they don't set a content-type - detect it
	// TODO: remove this after removing all the formats (too many tests to scrub til then)
	if writer.Header().Get("Content-Type") == "" {
		// see http.DetectContentType, the go server is supposed to do this for us but doesn't appear to?
		var contentType string
		jsonPrefix := [1]byte{'{'} // stack allocated
		if bytes.HasPrefix(writer.GetBuffer().Bytes(), jsonPrefix[:]) {
			// try to detect json, since DetectContentType isn't a hipster.
			contentType = "application/json; charset=utf-8"
		} else {
			contentType = http.DetectContentType(writer.GetBuffer().Bytes())
		}
		fmt.Printf("Setting: %s\n", contentType)
		writer.Header().Set("Content-Type", contentType)
	}
	fmt.Println(writer.Header())

	writer.Header().Set("Content-Length", strconv.Itoa(int(writer.GetBuffer().Len())))

	if writer.Status() > 0 {
		c.Writer.WriteHeader(writer.Status())
	}

	io.Copy(c.Writer, writer)

	return nil
}
