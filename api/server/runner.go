package server

import (
	"bytes"
	"context"
	"net/http"
	"path"
	"strings"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

type runnerResponse struct {
	RequestID string            `json:"request_id,omitempty"`
	Error     *models.ErrorBody `json:"error,omitempty"`
}

func (s *Server) handleRequest(c *gin.Context) {
	if strings.HasPrefix(c.Request.URL.Path, "/v1") {
		c.Status(http.StatusNotFound)
		return
	}

	ctx := c.Request.Context()

	r, routeExists := c.Get(api.Path)
	if !routeExists {
		r = "/"
	}

	reqRoute := &models.Route{
		AppName: c.MustGet(api.AppName).(string),
		Path:    path.Clean(r.(string)),
	}

	s.FireBeforeDispatch(ctx, reqRoute)

	s.serve(c, reqRoute.AppName, reqRoute.Path)

	s.FireAfterDispatch(ctx, reqRoute)
}

// TODO it would be nice if we could make this have nothing to do with the gin.Context but meh
// TODO make async store an *http.Request? would be sexy until we have different api format...
func (s *Server) serve(c *gin.Context, appName, path string) {
	// GetCall can mod headers, assign an id, look up the route/app (cached),
	// strip params, etc.
	call, err := s.Agent.GetCall(
		agent.WithWriter(c.Writer), // XXX (reed): order matters [for now]
		agent.FromRequest(appName, path, c.Request),
	)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	// TODO we could add FireBeforeDispatch right here with Call in hand

	if model := call.Model(); model.Type == "async" {
		// TODO we should push this into GetCall somehow (CallOpt maybe) or maybe agent.Queue(Call) ?
		buf := bytes.NewBuffer(make([]byte, 0, c.Request.ContentLength)) // TODO sync.Pool me
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

		if err == context.DeadlineExceeded {
			err = models.ErrCallTimeout // 504 w/ friendly note
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
