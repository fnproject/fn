package server

import (
	"bytes"
	"io"
	"strconv"

	"github.com/fnproject/cloudevent"
	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/common"
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

// handleTriggerHTTPFunctionCall2 executes the function and returns an error
// Requires the following in the context:
func (s *Server) handleFnInvokeCall2(c *gin.Context) error {
	fn, err := s.lbReadAccess.GetFnByID(c, c.Param(api.ParamFnID))
	if err != nil {
		return err
	}

	if fn.Format != models.FormatCloudEvent {
		// XXX(reed): we could just override it, as we do currently in the /r/ cloudevent model.
		// the blow up then gets pushed down into the container, which is arguably harder to chase (but fdks right?)
		return models.ErrOnlyCloudEventFnsSupported
	}

	app, err := s.lbReadAccess.GetAppByID(c, fn.AppID)
	if err != nil {
		return err
	}

	var event cloudevent.CloudEvent
	err = event.FromRequest(c.Request)
	if err != nil {
		// XXX(reed): we should make these API errors most likely instead of 5xx
		handleErrorResponse(c, err)
		return err
	}

	// TODO if we validate we have to strictly enforce certain fields we don't
	// really need in order to run a function (cloudEventVersion we do, but we
	// need to probably be lax and assume 0.1 if none provided, id doesn't
	// matter as for example, to run a function, we have call id)

	s.ServeFnInvoke(c, &event, fn, app)

	return nil
}

func (s *Server) ServeFnInvoke(c *gin.Context, event *cloudevent.CloudEvent, fn *models.Fn, app *models.App) error {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	writer := syncResponseWriter{
		Buffer:  buf,
		headers: c.Writer.Header(),
	}
	defer bufPool.Put(buf) // TODO need to ensure this is safe with Dispatch?

	call, err := s.agent.GetCall(
		agent.WithWriter(&writer), // XXX (reed): order matters [for now]
		agent.FromFnInvokeRequest(c.Request.Context(), app, fn, event),
	)

	// note that underneath, since we enforced the function format to be 'cloudevent'
	// then the container will produce a cloudevent (via the cloudevent protocol Dispatch method)
	// that will be in the writer after Submit gets done.

	err = s.agent.Submit(call)
	if err != nil {
		// NOTE if they cancel the request then it will stop the call (kind of cool),
		// we could filter that error out here too as right now it yells a little
		if err == models.ErrCallTimeoutServerBusy || err == models.ErrCallTimeout {
			// TODO maneuver
			// add this, since it means that start may not have been called [and it's relevant]
			// c.Writer.Header().Add("XXX-FXLB-WAIT", time.Now().Sub(time.Time(model.CreatedAt)).String())
		}
		return err
	}

	// TODO they may write out a binary cloud event to the buffer, we need to
	// handle this somehow (how do they set the headers?) but we can't set the
	// content type here really.
	// TODO we kinda need submit to return the event so we can do ^ b/c LAYERS

	writer.Header().Set("Content-Length", strconv.Itoa(int(buf.Len())))
	if writer.status > 0 {
		c.Writer.WriteHeader(writer.status)
	}
	io.Copy(c.Writer, &writer)

	return nil
}
