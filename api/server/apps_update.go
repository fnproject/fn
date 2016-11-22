package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/runner/common"
)

func handleAppUpdate(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)
	log := common.Logger(ctx)

	wapp := models.AppWrapper{}

	err := c.BindJSON(&wapp)
	if err != nil {
		log.WithError(err).Debug(models.ErrInvalidJSON)
		c.JSON(http.StatusBadRequest, simpleError(models.ErrInvalidJSON))
		return
	}

	if wapp.App == nil {
		log.Debug(models.ErrAppsMissingNew)
		c.JSON(http.StatusBadRequest, simpleError(models.ErrAppsMissingNew))
		return
	}

	wapp.App.Name = c.Param("app")
	app, err := Api.Datastore.UpdateApp(ctx, wapp.App)
	if err != nil {
		log.WithError(err).Debug(models.ErrAppsUpdate)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrAppsUpdate))
		return
	}

	wapp.App = app

	// Nothing to update right now in apps
	c.JSON(http.StatusOK, appResponse{"App successfully updated", wapp.App})
}
