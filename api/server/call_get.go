package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/fnproject/fn/api"
)

func (s *Server) handleCallGet(c *gin.Context) {
	ctx := c.Request.Context()

	callID := c.Param(api.Call)
	callObj, err := s.Datastore.GetTask(ctx, callID)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, fnCallResponse{"Successfully loaded call", callObj})
}
