package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleTriggerUpdate(c *gin.Context) {
	trigger := &models.Trigger{}

	err := c.BindJSON(trigger)
	if err != nil {
		if models.IsAPIError(err) {
			handleErrorResponse(c, err)
		} else {
			handleErrorResponse(c, models.ErrInvalidJSON)
		}
		return
	}

	pathTriggerID := c.Param(api.TriggerID)

	if trigger.ID == "" {
		trigger.ID = pathTriggerID
	} else {
		if pathTriggerID != trigger.ID {
			handleErrorResponse(c, models.ErrTriggerIDMismatch)
		}
	}

	ctx := c.Request.Context()
	triggerUpdated, err := s.datastore.UpdateTrigger(ctx, trigger)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, triggerUpdated)
}
