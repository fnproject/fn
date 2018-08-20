package server

import (
	"fmt"
	"net/http"

	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleTriggerCreate(c *gin.Context) {
	ctx := c.Request.Context()
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

	triggerCreated, err := s.datastore.InsertTrigger(ctx, trigger)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	if !s.noHTTTPTriggerEndpoint {
		app, err := s.datastore.GetAppByID(ctx, triggerCreated.AppID)
		if err != nil {
			handleErrorResponse(c, fmt.Errorf("unexpected error - trigger app not available: %s", err))
			return
		}

		triggerCreated, err = s.triggerAnnotator.AnnotateTrigger(c, app, triggerCreated)
		if err != nil {
			handleErrorResponse(c, err)
			return
		}
	}

	c.JSON(http.StatusOK, triggerCreated)
}
