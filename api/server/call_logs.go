package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleCallLogGet(c *gin.Context) {
	ctx := c.Request.Context()

	appName := c.MustGet(api.AppName).(string)
	callID := c.Param(api.Call)
	_, err := s.Datastore.GetCall(ctx, appName, callID)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	callObj, err := s.LogDB.GetLog(ctx, appName, callID)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, fnCallLogResponse{"Successfully loaded call", callObj})
}

func (s *Server) handleCallLogDelete(c *gin.Context) {
	ctx := c.Request.Context()

	appName := c.MustGet(api.AppName).(string)
	callID := c.Param(api.Call)
	_, err := s.Datastore.GetCall(ctx, appName, callID)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}
	err = s.LogDB.DeleteLog(ctx, appName, callID)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"message": "Log delete accepted"})
}
