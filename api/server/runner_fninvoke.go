package server

import (
	"bytes"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/event"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

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

const (
	fnInvokeContentType = "application/cloudevents+json"
)

// handleTriggerHTTPFunctionCall2 executes the function and returns an error
// Requires the following in the context:
func (s *Server) handleFnInvokeCall2(c *gin.Context) error {
	//	log := common.Logger(c.Request.Context())

	fn, err := s.lbReadAccess.GetFnByID(c, c.Param(api.ParamFnID))
	if err != nil {
		return err
	}

	if fn.Format != models.FormatCloudEvent {
		return models.ErrOnlyCloudEventFnsSupported
	}

	app, err := s.lbReadAccess.GetAppByID(c, fn.AppID)
	if err != nil {
		return err
	}

	contentType := c.ContentType()
	if contentType != fnInvokeContentType {
		return models.ErrUnsupportedMediaType
	}

	event := &event.Event{}
	err = c.BindJSON(event)
	if err != nil {
		if !models.IsAPIError(err) {
			err = models.ErrInvalidJSON
		}
		handleErrorResponse(c, err)
		return err
	}

	err = event.Validate()
	if err != nil {
		return err
	}

	s.ServeFnInvoke(c, event, fn, app)

	return nil
}

func (s *Server) ServeFnInvoke(c *gin.Context, event *event.Event, fn *models.Fn, app *models.App) error {
	log := common.Logger(c.Request.Context())
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	writer := syncResponseWriter{
		Buffer:  buf,
		headers: c.Writer.Header(),
	}
	defer bufPool.Put(buf) // TODO need to ensure this is safe with Dispatch?

	call, err := s.agent.GetCall(
		agent.WithWriter(&writer), // XXX (reed): order matters [for now]
		agent.FromFnInvokeRequest(c.Request.Context, app, fn, event),
	)

	return nil
}
