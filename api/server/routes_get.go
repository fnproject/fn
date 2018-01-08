package server

import (
	"net/http"
	"path"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleRouteGet(c *gin.Context) {
	ctx := c.Request.Context()

	appIDorName := c.MustGet(api.App).(string)
	initApp := &models.App{Name: appIDorName, ID: appIDorName}
	routePath := path.Clean("/" + c.MustGet(api.Path).(string))
	route, err := s.datastore.GetRoute(ctx, initApp, routePath)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, routeResponse{"Successfully loaded route", route})
}
