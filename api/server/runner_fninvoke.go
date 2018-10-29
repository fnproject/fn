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

const (
	InvokeSync   = "sync"
	InvokeDetach = "detach"
)

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
	fn, err := s.lbReadAccess.GetFnByID(c, c.Param(api.ParamFnID))
	if err != nil {
		return err
	}

	app, err := s.lbReadAccess.GetAppByID(c, fn.AppID)
	if err != nil {
		return err
	}

	// we use a querystring param to define the invoke as detached we can set a completely different
	// endpoint if we prefer.
	if c.Query("type") == InvokeDetach {
		return s.ServeFnInvokeDetached(c, app, fn)
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
	bufWriter := syncResponseWriter{
		headers: resp.Header(),
		status:  200,
		Buffer:  buf,
	}

	var writer http.ResponseWriter = &bufWriter
	writer = &jsonContentTypeTrapper{ResponseWriter: writer}

	opts := []agent.CallOpt{
		agent.WithWriter(writer), // XXX (reed): order matters [for now]
		agent.FromHTTPFnRequest(app, fn, req),
	}
	if trig != nil {
		opts = append(opts, agent.WithTrigger(trig))
	}

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

	// buffered response writer traps status (so we can add headers), we need to write it still
	if bufWriter.status > 0 {
		resp.WriteHeader(bufWriter.status)
	}

	io.Copy(resp, buf)
	bufPool.Put(buf) // at this point, submit returned without timing out, so we can re-use this one
	return nil
}

// TODO kill this thing after removing tests for http/json/default formats
type jsonContentTypeTrapper struct {
	http.ResponseWriter
	committed bool
}

var _ http.ResponseWriter = new(jsonContentTypeTrapper) // nice compiler errors

func (j *jsonContentTypeTrapper) Write(b []byte) (int, error) {
	if !j.committed {
		// override default content type detection behavior to add json
		j.detectContentType(b)
	}
	j.committed = true

	// write inner
	return j.ResponseWriter.Write(b)
}

func (j *jsonContentTypeTrapper) detectContentType(b []byte) {
	if j.Header().Get("Content-Type") == "" {
		// see http.DetectContentType
		var contentType string
		jsonPrefix := [1]byte{'{'} // stack allocated
		if bytes.HasPrefix(b, jsonPrefix[:]) {
			// try to detect json, since DetectContentType isn't a hipster.
			contentType = "application/json; charset=utf-8"
		} else {
			contentType = http.DetectContentType(b)
		}
		j.Header().Set("Content-Type", contentType)
	}
}
