package server

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"sync"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/tag"
)

var (
	bufPool = &sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}
)

// ResponseBuffer  implements http.ResponseWriter
type ResponseBuffer interface {
	http.ResponseWriter
	Status() int
}

// implements http.ResponseWriter
// this little guy buffers responses from user containers and lets them still
// set headers and such without us risking writing partial output [as much, the
// server could still die while we're copying the buffer]. this lets us set
// content length and content type nicely, as a bonus. it is sad, yes.
type syncResponseWriter struct {
	headers http.Header
	status  int
	*bytes.Buffer
}

var _ http.ResponseWriter = new(syncResponseWriter) // nice compiler errors

func (s *syncResponseWriter) Header() http.Header  { return s.headers }
func (s *syncResponseWriter) WriteHeader(code int) { s.status = code }
func (s *syncResponseWriter) Status() int          { return s.status }

// handleFnInvokeCall executes the function, for router handlers
func (s *Server) handleFnInvokeCall(c *gin.Context) {
	fnID := c.Param(api.FnID)
	ctx, _ := common.LoggerWithFields(c.Request.Context(), logrus.Fields{"fn_id": fnID})
	c.Request = c.Request.WithContext(ctx)
	err := s.handleFnInvokeCall2(c)
	if err != nil {
		handleErrorResponse(c, err)
	}
}

// handleTriggerHTTPFunctionCall2 executes the function and returns an error
// Requires the following in the context:
func (s *Server) handleFnInvokeCall2(c *gin.Context) error {
	ctx := c.Request.Context()
	fn, err := s.lbReadAccess.GetFnByID(ctx, c.Param(api.FnID))
	if err != nil {
		return err
	}

	app, err := s.lbReadAccess.GetAppByID(ctx, fn.AppID)
	if err != nil {
		return err
	}

	err = s.ServeFnInvoke(c, app, fn)
	if models.IsFuncError(err) || err == nil {
		// report all user-directed errors and function responses from here, after submit has run.
		// this is our never ending attempt to distinguish user and platform errors.
		ctx, err := tag.New(ctx,
			tag.Insert(whodunitKey, "user"),
		)
		if err != nil {
			panic(err)
		}
		c.Request = c.Request.WithContext(ctx)
	}
	return err
}

func (s *Server) ServeFnInvoke(c *gin.Context, app *models.App, fn *models.Fn) error {
	return s.fnInvoke(c.Writer, c.Request, app, fn, nil)
}

func (s *Server) fnInvoke(resp http.ResponseWriter, req *http.Request, app *models.App, fn *models.Fn, trig *models.Trigger) error {
	// TODO: we should get rid of the buffers, and stream back (saves memory (+splice), faster (splice), allows streaming, don't have to cap resp size)
	// buffer the response before writing it out to client to prevent partials from trying to stream
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()

	var opts []agent.CallOpt
	opts = append(opts, agent.FromHTTPFnRequest(app, fn, req))

	var writer ResponseBuffer
	isDetached := req.Header.Get("Fn-Invoke-Type") == models.TypeDetached
	if isDetached {
		writer = agent.NewDetachedResponseWriter(202)
		opts = append(opts, agent.InvokeDetached())
	} else {
		writer = &syncResponseWriter{
			headers: resp.Header(),
			status:  200,
			Buffer:  buf,
		}
	}

	opts = append(opts, agent.WithWriter(writer))
	opts = append(opts, agent.WithStderrLogger())
	if trig != nil {
		opts = append(opts, agent.WithTrigger(trig))
	}

	call, err := s.agent.GetCall(opts...)
	if err != nil {
		return err
	}

	// add this before submit, always tie a call id to the response at this point
	writer.Header().Add("Fn-Call-Id", call.Model().ID)

	err = s.agent.Submit(call)
	if err != nil {
		return err
	}

	// because we can...
	writer.Header().Set("Content-Length", strconv.Itoa(int(buf.Len())))

	// buffered response writer traps status (so we can add headers), we need to write it still
	if writer.Status() > 0 {
		resp.WriteHeader(writer.Status())
	}

	if isDetached {
		return nil
	}

	io.Copy(resp, buf)
	bufPool.Put(buf) // at this point, submit returned without timing out, so we can re-use this one
	return nil
}
