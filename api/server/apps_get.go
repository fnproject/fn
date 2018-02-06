package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleAppGetByName(c *gin.Context) {
	ctx := c.Request.Context()

	appName := c.MustGet(api.App).(string)

	app, err := s.datastore.GetAppByName(ctx, appName)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, appResponse{"Successfully loaded app", app})
}

func (s *Server) handleAppGetByID(c *gin.Context) {
	ctx := c.Request.Context()

	app, err := s.datastore.GetAppByName(ctx, c.MustGet(api.AppID).(string))
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, appResponse{"Successfully loaded app", app})
}
