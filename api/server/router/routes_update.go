package router

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
)

func handleRouteUpdate(c *gin.Context) {
	store := c.MustGet("store").(models.Datastore)
	log := c.MustGet("log").(logrus.FieldLogger)

	wroute := &models.RouteWrapper{}
	appName := c.Param("app")

	err := c.BindJSON(wroute)
	if err != nil {
		log.WithError(err).Debug(models.ErrInvalidJSON)
		c.JSON(http.StatusBadRequest, simpleError(models.ErrInvalidJSON))
		return
	}

	wroute.Route.AppName = appName

	route, err := store.StoreRoute(wroute.Route)
	if err != nil {
		log.WithError(err).Debug(models.ErrAppsCreate)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrAppsCreate))
		return
	}

	c.JSON(http.StatusOK, route)
}
