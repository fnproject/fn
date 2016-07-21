package router

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
)

func handleAppDestroy(c *gin.Context) {
	store := c.MustGet("store").(models.Datastore)
	log := c.MustGet("log").(logrus.FieldLogger)

	appName := c.Param("app")
	err := store.RemoveApp(appName)

	if err != nil {
		log.WithError(err).Debug(models.ErrAppsRemoving)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrAppsRemoving))
		return
	}

	c.JSON(http.StatusOK, nil)
}
