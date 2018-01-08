package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleCallGet(c *gin.Context) {
	ctx := c.Request.Context()

	appIDorName := c.MustGet(api.App).(string)
	callID := c.Param(api.Call)
	callObj, err := s.datastore.GetCall(ctx, appIDorName, callID)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, callResponse{"Successfully loaded call", callObj})
}
