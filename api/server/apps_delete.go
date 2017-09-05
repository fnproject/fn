package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleAppDelete(c *gin.Context) {
	ctx := c.Request.Context()
	log := common.Logger(ctx)

	app := &models.App{Name: c.MustGet(api.AppName).(string)}

	routes, err := s.Datastore.GetRoutesByApp(ctx, app.Name, &models.RouteFilter{})
	if err != nil {
		log.WithError(err).Error("error getting route in app delete")
		handleErrorResponse(c, err)
		return
	}
	//TODO allow this? #528
	if len(routes) > 0 {
		handleErrorResponse(c, models.ErrDeleteAppsWithRoutes)
		return
	}

	err = s.FireBeforeAppDelete(ctx, app)
	if err != nil {
		log.WithError(err).Error("error firing before app delete")
		handleErrorResponse(c, err)
		return
	}

	app, err = s.Datastore.GetApp(ctx, app.Name)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	err = s.Datastore.RemoveApp(ctx, app.Name)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	err = s.FireAfterAppDelete(ctx, app)
	if err != nil {
		log.WithError(err).Error("error firing after app delete")
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "App deleted"})
}
