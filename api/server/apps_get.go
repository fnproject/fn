package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/runner/common"
)

func handleAppGet(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)
	log := common.Logger(ctx)

	appName := c.Param("app")
	app, err := Api.Datastore.GetApp(ctx, appName)

	if err != nil && err != models.ErrAppsNotFound {
		log.WithError(err).Error(models.ErrAppsGet)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrAppsGet))
		return
	} else if app == nil {
		log.WithError(err).Error(models.ErrAppsNotFound)
		c.JSON(http.StatusNotFound, simpleError(models.ErrAppsNotFound))
		return
	}

	c.JSON(http.StatusOK, appResponse{"Successfully loaded app", app})
}
