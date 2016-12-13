package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/runner/common"
)

func (s *Server) handleAppDelete(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)
	log := common.Logger(ctx)

	app := &models.App{Name: ctx.Value("appName").(string)}

	routes, err := s.Datastore.GetRoutesByApp(ctx, app.Name, &models.RouteFilter{})
	if err != nil {
		log.WithError(err).Debug(models.ErrAppsRemoving)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrAppsRemoving))
		return
	}

	if len(routes) > 0 {
		log.WithError(err).Debug(models.ErrDeleteAppsWithRoutes)
		c.JSON(http.StatusBadRequest, simpleError(models.ErrDeleteAppsWithRoutes))
		return
	}

	err = s.FireBeforeAppDelete(ctx, app)
	if err != nil {
		log.WithError(err).Errorln(models.ErrAppsRemoving)
		c.JSON(http.StatusInternalServerError, simpleError(err))
		return
	}

	if err = s.Datastore.RemoveApp(ctx, app.Name); err != nil {
		log.WithError(err).Debug(models.ErrAppsRemoving)
		if err == models.ErrAppsNotFound {
			c.JSON(http.StatusNotFound, simpleError(models.ErrAppsNotFound))
		} else {
			c.JSON(http.StatusInternalServerError, simpleError(models.ErrAppsRemoving))
		}
		return
	}

	err = s.FireAfterAppDelete(ctx, app)
	if err != nil {
		log.WithError(err).Errorln(models.ErrAppsRemoving)
		c.JSON(http.StatusInternalServerError, simpleError(err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "App deleted"})
}
