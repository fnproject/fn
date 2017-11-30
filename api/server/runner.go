package server

import (
	"bytes"
	"errors"
	"net/http"
	"path"
	"time"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// handleFunctionCall executes the function.
// Requires the following in the context:
// * "app_name"
// * "path"
func (s *Server) handleFunctionCall(c *gin.Context) {
	ctx := c.Request.Context()
	var p string
	r := ctx.Value(api.Path)
	if r == nil {
		p = "/"
	} else {
		p = r.(string)
	}

	var a string
	ai := ctx.Value(api.AppName)
	if ai == nil {
		handleErrorResponse(c, errors.New("app name not set"))
		return
	}
	a = ai.(string)

	s.serve(c, a, path.Clean(p))
}

// convert gin.Params to agent.Params to avoid introducing gin
// dependency to agent
func parseParams(params gin.Params) agent.Params {
	out := make(agent.Params, 0, len(params))
	for _, val := range params {
		out = append(out, agent.Param{Key: val.Key, Value: val.Value})
	}
	return out
}

// TODO it would be nice if we could make this have nothing to do with the gin.Context but meh
// TODO make async store an *http.Request? would be sexy until we have different api format...
func (s *Server) serve(c *gin.Context, appName, path string) {
	// GetCall can mod headers, assign an id, look up the route/app (cached),
	// strip params, etc.
	call, err := s.Agent.GetCall(
		agent.WithWriter(c.Writer), // XXX (reed): order matters [for now]
		agent.FromRequest(appName, path, c.Request, parseParams(c.Params)),
	)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	model := call.Model()
	{ // scope this, to disallow ctx use outside of this scope. add id for handleErrorResponse logger
		ctx, _ := common.LoggerWithFields(c.Request.Context(), logrus.Fields{"id": model.ID})
		c.Request = c.Request.WithContext(ctx)
	}

	if model.Type == "async" {
		// TODO we should push this into GetCall somehow (CallOpt maybe) or maybe agent.Queue(Call) ?
		contentLength := c.Request.ContentLength
		if contentLength < 128 { // contentLength could be -1 or really small, sanitize
			contentLength = 128
		}
		buf := bytes.NewBuffer(make([]byte, int(contentLength))[:0]) // TODO sync.Pool me
		_, err := buf.ReadFrom(c.Request.Body)
		if err != nil {
			handleErrorResponse(c, models.ErrInvalidPayload)
			return
		}
		model.Payload = buf.String()

		// TODO we should probably add this to the datastore too. consider the plumber!
		_, err = s.MQ.Push(c.Request.Context(), model)
		if err != nil {
			handleErrorResponse(c, err)
			return
		}

		c.JSON(http.StatusAccepted, map[string]string{"call_id": model.ID})
		return
	}

	err = s.Agent.Submit(call)
	if err != nil {
		// NOTE if they cancel the request then it will stop the call (kind of cool),
		// we could filter that error out here too as right now it yells a little
		if err == models.ErrCallTimeoutServerBusy || err == models.ErrCallTimeout {
			// TODO maneuver
			// add this, since it means that start may not have been called [and it's relevant]
			c.Writer.Header().Add("XXX-FXLB-WAIT", time.Now().Sub(time.Time(model.CreatedAt)).String())
		}
		// NOTE: if the task wrote the headers already then this will fail to write
		// a 5xx (and log about it to us) -- that's fine (nice, even!)
		handleErrorResponse(c, err)
		return
	}

	// TODO plumb FXLB-WAIT somehow (api?)

	// TODO we need to watch the response writer and if no bytes written
	// then write a 200 at this point?
	// c.Data(http.StatusOK)
}
