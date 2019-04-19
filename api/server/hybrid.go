package server

import (
	"strings"

	"errors"
	"fmt"
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

// TODO: figure out what to do with this, stale interface from hybrid days but still in use
func (s *Server) handleRunnerGetTriggerBySource(c *gin.Context) {
	ctx := c.Request.Context()

	appID := c.Param(api.AppID)

	triggerType := c.Param(api.TriggerType)
	if triggerType == "" {
		handleErrorResponse(c, errors.New("no trigger type in request"))
		return
	}
	triggerSource := strings.TrimPrefix(c.Param(api.TriggerSource), "/")

	trigger, err := s.datastore.GetTriggerBySource(ctx, appID, triggerType, triggerSource)

	if err != nil {
		handleErrorResponse(c, err)
		return
	}
	// Not clear that we really need to annotate the trigger here but ... lets do it just in case.
	app, err := s.datastore.GetAppByID(ctx, trigger.AppID)

	if err != nil {
		handleErrorResponse(c, fmt.Errorf("unexpected error - trigger app not available: %s", err))
	}

	s.triggerAnnotator.AnnotateTrigger(c, app, trigger)

	c.JSON(http.StatusOK, trigger)
}
