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
	if c.Request.Method != http.MethodPost {
		handleErrorResponse(c, models.ErrInvokePostOnly)
		return
	}
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
	return s.fnInvoke(c.Writer, c.Request, app, fn, nil)
}

func (s *Server) fnInvoke(resp http.ResponseWriter, req *http.Request, app *models.App, fn *models.Fn, trig *models.Trigger) error {
	// TODO: we should get rid of the buffers, and stream back (saves memory (+splice), faster (splice), allows streaming, don't have to cap resp size)
	// buffer the response before writing it out to client to prevent partials from trying to stream
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	var writer ResponseBuffer

	isDetached := req.Header.Get("Fn-Invoke-Type") == models.TypeDetached
	if isDetached {
		writer = agent.NewDetachedResponseWriter(resp.Header(), 202)
	} else {
		writer = &syncResponseWriter{
			headers: resp.Header(),
			status:  200,
			Buffer:  buf,
		}
	}
	opts := getCallOptions(req, app, fn, trig, writer)

	call, err := s.agent.GetCall(opts...)
	if err != nil {
		return err
	}

	err = s.agent.Submit(call)
	if err != nil {
		return err
	}

	// because we can...
	writer.Header().Set("Content-Length", strconv.Itoa(int(buf.Len())))
	writer.Header().Add("Fn-Call-Id", call.Model().ID) // XXX(reed): move to before Submit when adding streaming

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

func getCallOptions(req *http.Request, app *models.App, fn *models.Fn, trig *models.Trigger, rw http.ResponseWriter) []agent.CallOpt {
	var opts []agent.CallOpt
	opts = append(opts, agent.WithWriter(rw)) // XXX (reed): order matters [for now]
	opts = append(opts, agent.FromHTTPFnRequest(app, fn, req))

	if req.Header.Get("Fn-Invoke-Type") == models.TypeDetached {
		opts = append(opts, agent.InvokeDetached())
	}

	if trig != nil {
		opts = append(opts, agent.WithTrigger(trig))
	}
	return opts
}
