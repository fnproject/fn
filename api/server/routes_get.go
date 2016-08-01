package server

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
)

func handleRouteGet(c *gin.Context) {
	log := c.MustGet("log").(logrus.FieldLogger)

	appName := c.Param("app")
	routeName := c.Param("route")

	route, err := Api.Datastore.GetRoute(appName, routeName)
	if err != nil {
		log.WithError(err).Error(models.ErrRoutesGet)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrRoutesGet))
		return
	}

	if route == nil {
		log.Error(models.ErrRoutesNotFound)
		c.JSON(http.StatusNotFound, simpleError(models.ErrRoutesNotFound))
		return
	}

	log.WithFields(logrus.Fields{"route": route}).Debug("Got route")

	c.JSON(http.StatusOK, &models.RouteWrapper{route})
}
