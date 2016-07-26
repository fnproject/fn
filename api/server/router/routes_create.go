package router

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
)

func handleRouteCreate(c *gin.Context) {
	store := c.MustGet("store").(models.Datastore)
	log := c.MustGet("log").(logrus.FieldLogger)

	wroute := &models.RouteWrapper{}

	err := c.BindJSON(wroute)
	if err != nil {
		log.WithError(err).Error(models.ErrInvalidJSON)
		c.JSON(http.StatusBadRequest, simpleError(models.ErrInvalidJSON))
		return
	}

	if wroute.Route == nil {
		log.WithError(err).Error(models.ErrInvalidJSON)
		c.JSON(http.StatusBadRequest, simpleError(models.ErrRoutesMissingNew))
		return
	}

	wroute.Route.AppName = c.Param("app")

	if err := wroute.Validate(); err != nil {
		log.Error(err)
		c.JSON(http.StatusInternalServerError, simpleError(err))
		return
	}

	app, err := store.GetApp(wroute.Route.AppName)
	if err != nil {
		log.WithError(err).Error(models.ErrAppsGet)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrAppsGet))
		return
	}
	if app == nil {
		app, err = store.StoreApp(&models.App{Name: wroute.Route.AppName})
		if err != nil {
			log.WithError(err).Error(models.ErrAppsCreate)
			c.JSON(http.StatusInternalServerError, simpleError(models.ErrAppsCreate))
			return
		}
	}

	route, err := store.StoreRoute(wroute.Route)
	if err != nil {
		log.WithError(err).Error(models.ErrRoutesCreate)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrRoutesCreate))
		return
	}

	c.JSON(http.StatusOK, route)
}
