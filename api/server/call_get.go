package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"gitlab-odx.oracle.com/odx/functions/api"
)

func (s *Server) handleCallGet(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)

	callID := c.Param(api.Call)
	callObj, err := s.Datastore.GetTask(ctx, callID)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, fnCallResponse{"Successfully loaded call", callObj})
}
