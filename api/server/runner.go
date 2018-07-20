package server

import (
	"bytes"
	"net/http"
	"path"
	"sync"
	"time"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/event/httpevent"
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
	defer bufPool.Put(buf)
	ctx := c.Request.Context()
	log := common.Logger(ctx)
	evt, err := httpevent.FromHTTPRequest(c.Request, 1024*1024)

	if err != nil {
		return err
	}

	call, err := s.agent.GetCall(ctx,
		agent.FromRouteAndEvent(app, route, evt),
	)
	if err != nil {
		return err
	}
	model := call.Model()
	{ // scope this, to disallow ctx use outside of this scope. add id for handleV1ErrorResponse logger
		ctx, _ = common.LoggerWithFields(ctx, logrus.Fields{"id": model.ID})
	}

	if model.Type == "async" {
		err = s.lbEnqueue.Enqueue(ctx, model)
		if err != nil {
			return err
		}

		c.JSON(http.StatusAccepted, map[string]string{"call_id": model.ID})
		return nil
	}

	outEvt, err := s.agent.Submit(ctx, call)
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

	err = httpevent.WriteHTTPResponse(ctx, outEvt, c.Writer)
	if err != nil {
		log.WithError(err).Error("Failed to write HTTP response")
	}

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
