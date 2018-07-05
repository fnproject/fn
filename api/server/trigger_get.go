package server

import (
	"fmt"
	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
	"net/http"
)

func (s *Server) handleTriggerGet(c *gin.Context) {
	ctx := c.Request.Context()

	trigger, err := s.datastore.GetTriggerByID(ctx, c.Param(api.ParamTriggerID))

	if err != nil {
		handleErrorResponse(c, err)
		return
	}
	app, err := s.datastore.GetAppByID(ctx, trigger.AppID)

	if err != nil {
		handleErrorResponse(c, fmt.Errorf("unexpected error - trigger app not available: %s", err))
	}

	s.triggerAnnotator.AnnotateTrigger(c, app, trigger)

	c.JSON(http.StatusOK, trigger)
}
