package server

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
)

func handleRouteDelete(c *gin.Context) {
	log := c.MustGet("log").(logrus.FieldLogger)

	appName := c.Param("app")
	routeName := c.Param("route")
	err := Api.Datastore.RemoveRoute(appName, routeName)

	if err != nil {
		log.WithError(err).Debug(models.ErrRoutesRemoving)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrRoutesRemoving))
		return
	}

	c.JSON(http.StatusOK, nil)
}
