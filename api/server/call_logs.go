package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleCallLogGet(c *gin.Context) {
	ctx := c.Request.Context()

	callID := c.Param(api.Call)
	_, err := s.Datastore.GetTask(ctx, callID)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	callObj, err := s.LogDB.GetLog(ctx, callID)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, fnCallLogResponse{"Successfully loaded call", callObj})
}

func (s *Server) handleCallLogDelete(c *gin.Context) {
	ctx := c.Request.Context()

	callID := c.Param(api.Call)
	_, err := s.Datastore.GetTask(ctx, callID)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}
	err = s.LogDB.DeleteLog(ctx, callID)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"message": "Log delete accepted"})
}
