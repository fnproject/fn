package router

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
)

func handleAppList(c *gin.Context) {
	store := c.MustGet("store").(models.Datastore)
	log := c.MustGet("log").(logrus.FieldLogger)

	filter := &models.AppFilter{}

	apps, err := store.GetApps(filter)
	if err != nil {
		log.WithError(err).Debug(models.ErrAppsList)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrAppsList))
		return
	}

	c.JSON(http.StatusOK, &models.AppsWrapper{apps})
}
