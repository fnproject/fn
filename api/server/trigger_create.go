package server

import (
	"net/http"

	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleTriggerCreate(c *gin.Context) {
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

	if err = trigger.ValidCreate(); err != nil {
		handleErrorResponse(c, err)
		return
	}

	triggerUpdated, err := s.datastore.InsertTrigger(c, trigger)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, triggerResponse{"Trigger successfully created", triggerUpdated})
}
