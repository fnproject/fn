package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api"
	"github.com/iron-io/functions/api/models"
)

func (s *Server) handleRouteList(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)

	filter := &models.RouteFilter{}

	if img := c.Query("image"); img != "" {
		filter.Image = img
	}

	var routes []*models.Route
	var err error
	if appName, ok := c.MustGet(api.AppName).(string); ok && appName != "" {
		routes, err = s.Datastore.GetRoutesByApp(ctx, appName, filter)
	} else {
		routes, err = s.Datastore.GetRoutes(ctx, filter)
	}

	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, routesResponse{"Sucessfully listed routes", routes})
}
