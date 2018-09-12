package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleCallGet(c *gin.Context) {
	ctx := c.Request.Context()

	callID := c.Param(api.ParamCallID)

	callObj, err := s.logstore.GetCall(ctx, callID)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, callObj)
}
