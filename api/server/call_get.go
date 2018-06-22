package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleCallGet(c *gin.Context) {
	ctx := c.Request.Context()

	callID := c.Param(api.Call)
	appID := c.MustGet(api.AppID).(string)

	callObj, err := s.logstore.GetCall(ctx, appID, callID)
	if err != nil {
		handleV1ErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, callResponse{"Successfully loaded call", callObj})
}
