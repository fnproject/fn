package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleAppGetByID(c *gin.Context) {
	ctx := c.Request.Context()

	app, err := s.datastore.GetAppByID(ctx, c.Param(api.AppID))
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, app)
}
