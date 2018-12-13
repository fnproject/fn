package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleAppGet(c *gin.Context) {
	ctx := c.Request.Context()

	appId := c.Param(api.AppID)
	app, err := s.datastore.GetAppByID(ctx, appId)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, app)
}
