package server

import (
	"context"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/runner/common"
)

func handleRouteGet(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)
	log := common.Logger(ctx)

	appName := c.Param("app")
	routePath := c.Param("route")

	route, err := Api.Datastore.GetRoute(appName, routePath)
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

	c.JSON(http.StatusOK, routeResponse{"Successfully loaded route", route})
}
