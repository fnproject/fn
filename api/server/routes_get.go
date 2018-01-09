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

	initApp := &models.App{Name: c.MustGet(api.App).(string)}
	routePath := path.Clean("/" + c.MustGet(api.Path).(string))
	app, err := s.datastore.GetApp(ctx, initApp)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}
	route, err := s.datastore.GetRoute(ctx, app, routePath)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, routeResponse{"Successfully loaded route", route})
}
