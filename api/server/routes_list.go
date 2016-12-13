package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/runner/common"
)

func (s *Server) handleRouteList(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)
	log := common.Logger(ctx)

	filter := &models.RouteFilter{}

	if img := c.Query("image"); img != "" {
		filter.Image = img
	}

	var routes []*models.Route
	var err error
	if appName, ok := ctx.Value("appName").(string); ok && appName != "" {
		routes, err = s.Datastore.GetRoutesByApp(ctx, appName, filter)
	} else {
		routes, err = s.Datastore.GetRoutes(ctx, filter)
	}

	if err == models.ErrAppsNotFound {
		log.WithError(err).Debug(models.ErrRoutesGet)
		c.JSON(http.StatusNotFound, simpleError(err))
		return
	} else if err != nil {
		log.WithError(err).Error(models.ErrRoutesGet)
		c.JSON(http.StatusInternalServerError, simpleError(ErrInternalServerError))
		return
	}

	c.JSON(http.StatusOK, routesResponse{"Sucessfully listed routes", routes})
}
