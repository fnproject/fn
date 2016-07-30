package router

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
)

func handleAppGet(c *gin.Context) {
	store := c.MustGet("store").(models.Datastore)
	log := c.MustGet("log").(logrus.FieldLogger)

	appName := c.Param("app")
	app, err := store.GetApp(appName)

	if err != nil {
		log.WithError(err).Error(models.ErrAppsGet)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrAppsGet))
		return
	}

	if app == nil {
		log.WithError(err).Error(models.ErrAppsNotFound)
		c.JSON(http.StatusNotFound, simpleError(models.ErrAppsNotFound))
		return
	}

	c.JSON(http.StatusOK, &models.AppWrapper{app})
}
