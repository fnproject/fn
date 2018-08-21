package server

import (
	"bytes"
	"io"
	"net/http"
	"path"
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

// handleV1FunctionCall executes the function, for router handlers
func (s *Server) handleV1FunctionCall(c *gin.Context) {
	err := s.handleFunctionCall2(c)
	if err != nil {
		handleV1ErrorResponse(c, err)
	}
}

// handleFunctionCall2 executes the function and returns an error
// Requires the following in the context:
// * "app"
// * "path"
func (s *Server) handleFunctionCall2(c *gin.Context) error {
	ctx := c.Request.Context()
	var p string
	r := PathFromContext(ctx)
	if r == "" {
		p = "/"
	} else {
		p = r
	}

	appID := c.MustGet(api.AppID).(string)
	app, err := s.lbReadAccess.GetAppByID(ctx, appID)
	if err != nil {
		return err
	}

	routePath := path.Clean(p)
	route, err := s.lbReadAccess.GetRoute(ctx, appID, routePath)
	if err != nil {
		return err
	}
	// gin sets this to 404 on NoRoute, so we'll just ensure it's 200 by default.
	c.Status(200) // this doesn't write the header yet

	return s.ServeRoute(c, app, route)
}

var (
	bufPool = &sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}
)

// ServeRoute serves an HTTP route for a given app
// This is exported to allow extensions to plugin their own route handling
func (s *Server) ServeRoute(c *gin.Context, app *models.App, route *models.Route) error {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	writer := syncResponseWriter{
		Buffer:  buf,
		headers: c.Writer.Header(), // copy ref
	}
	defer bufPool.Put(buf) // TODO need to ensure this is safe with Dispatch?

	// GetCall can mod headers, assign an id, look up the route/app (cached),
	// strip params, etc.
	// this should happen ASAP to turn app name to app ID

	// GetCall can mod headers, assign an id, look up the route/app (cached),
	// strip params, etc.

	ctx := c.Request.Context()

	call, err := s.agent.GetCall(ctx,
		agent.WithWriter(&writer), // XXX (reed): order matters [for now]
		agent.FromRequest(app, route, c.Request),
	)
	if err != nil {
		return err
	}
	model := call.Model()
	{ // scope this, to disallow ctx use outside of this scope. add id for handleV1ErrorResponse logger
		ctx, _ = common.LoggerWithFields(ctx, logrus.Fields{"id": model.ID})
	}

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

	err = s.agent.Submit(ctx, call)
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

var _ http.ResponseWriter = new(syncResponseWriter)

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

func (s *syncResponseWriter) Header() http.Header  { return s.headers }
func (s *syncResponseWriter) WriteHeader(code int) { s.status = code }
