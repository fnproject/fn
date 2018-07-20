package server

import (
	"bytes"
	"net/http"
	"time"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/event/httpevent"
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

//ServeHTTPTrigger serves an HTTP trigger for a given app/fn/trigger  based on the current request
// This is exported to allow extensions to handle their own trigger naming and publishing
func (s *Server) ServeHTTPTrigger(c *gin.Context, app *models.App, fn *models.Fn, trigger *models.Trigger) error {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf) // TODO need to ensure this is safe with Dispatch?

	ctx := c.Request.Context()
	log := common.Logger(ctx)
	// TODO cap max request size
	inputEvent, err := httpevent.FromHTTPRequest(c.Request, 1024*1024)
	if err != nil {
		return err
	}
	call, err := s.agent.GetCall(ctx,
		agent.FromEvent(app, fn, inputEvent),
	)
	if err != nil {
		return err
	}
	model := call.Model()
	{ // scope this, to disallow ctx use outside of this scope. add id for handleV1ErrorResponse logger
		ctx, _ = common.LoggerWithFields(ctx, logrus.Fields{"id": model.ID})
	}

	// TODO TRIGGERWIP  not clear this makes sense here - but it works  so...
	if model.Type == "async" {
		err = s.lbEnqueue.Enqueue(ctx, model)
		if err != nil {
			return err
		}
		c.JSON(http.StatusAccepted, map[string]string{"call_id": model.ID})
		return nil
	}

	respEvent, err := s.agent.Submit(ctx, call)
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
	err = httpevent.WriteHTTPResponse(ctx, respEvent, c.Writer)
	log.WithError(err).Error("Failed to write HTTP response event ")
	return nil
}
