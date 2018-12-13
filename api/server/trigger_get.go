package server

import (
	"fmt"
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleTriggerGet(c *gin.Context) {
	ctx := c.Request.Context()

	trigger, err := s.datastore.GetTriggerByID(ctx, c.Param(api.TriggerID))

	if err != nil {
		handleErrorResponse(c, err)
		return
	}
	app, err := s.datastore.GetAppByID(ctx, trigger.AppID)

	if err != nil {
		handleErrorResponse(c, fmt.Errorf("unexpected error - trigger app not available: %s", err))
		return
	}

	trigger, err = s.triggerAnnotator.AnnotateTrigger(c, app, trigger)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, trigger)
}
