package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleCallGet1(c *gin.Context) {
	ctx := c.Request.Context()

	callID := c.Param(api.ParamCallID)
	appID := c.MustGet(api.AppID).(string)

	callObj, err := s.logstore.GetCall1(ctx, appID, callID)
	if err != nil {
		handleV1ErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, callResponse{"Successfully loaded call", callObj})
}

func (s *Server) handleCallGet(c *gin.Context) {
	ctx := c.Request.Context()

	fnID := c.Param(api.ParamFnID)
	callID := c.Param(api.ParamCallID)

	callObj, err := s.logstore.GetCall(ctx, fnID, callID)
	if err != nil {
		handleV1ErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, callResponse{"Successfully loaded call", callObj})
}
