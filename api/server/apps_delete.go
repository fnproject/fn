package server

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
)

func handleAppDelete(c *gin.Context) {
	log := c.MustGet("log").(logrus.FieldLogger)

	appName := c.Param("app")
	err := Api.Datastore.RemoveApp(appName)

	if err != nil {
		log.WithError(err).Debug(models.ErrAppsRemoving)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrAppsRemoving))
		return
	}

	c.JSON(http.StatusOK, nil)
}
