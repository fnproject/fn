package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleTriggerDelete(c *gin.Context) {
	ctx := c.Request.Context()

	err := s.datastore.RemoveTrigger(ctx, c.Param(api.TriggerID))
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.String(http.StatusNoContent, "")
}
