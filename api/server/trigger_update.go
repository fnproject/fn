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
			handleV1ErrorResponse(c, err)
		} else {
			handleV1ErrorResponse(c, models.ErrInvalidJSON)
		}
		return
	}

	pathTriggerID := c.Param(api.TriggerID)

	if trigger.ID == "" {
		trigger.ID = pathTriggerID
	} else {
		if pathTriggerID != trigger.ID {
			handleV1ErrorResponse(c, models.ErrIDMismatch)
		}
	}

	triggerUpdated, err := s.datastore.UpdateTrigger(c, trigger)
	if err != nil {
		handleV1ErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, triggerUpdated)
}
