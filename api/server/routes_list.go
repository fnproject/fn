package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gitlab-odx.oracle.com/odx/functions/api"
	"gitlab-odx.oracle.com/odx/functions/api/models"
)

func (s *Server) handleRouteList(c *gin.Context) {
	ctx := c.Request.Context()

	filter := &models.RouteFilter{}

	if img := c.Query("image"); img != "" {
		filter.Image = img
	}

	var routes []*models.Route
	var err error
	appName, exists := c.Get(api.AppName)
	name, ok := appName.(string)
	if exists && ok && name != "" {
		routes, err = s.Datastore.GetRoutesByApp(ctx, name, filter)
		// if there are no routes for the app, check if the app exists to return 404 if it does not
		if len(routes) == 0 {
			_, err = s.Datastore.GetApp(ctx, name)
		}
	} else {
		routes, err = s.Datastore.GetRoutes(ctx, filter)
	}

	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, routesResponse{"Successfully listed routes", routes})
}
