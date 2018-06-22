package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleTriggerGet(c *gin.Context) {
	ctx := c.Request.Context()

	trigger, err := s.datastore.GetTriggerByID(ctx, c.Param(api.TriggerID))
	if err != nil {
		handleV1ErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, trigger)
}
