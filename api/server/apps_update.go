package server

import (
	"net/http"

	"golang.org/x/net/context"

	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	titancommon "github.com/iron-io/worker/common"
)

func handleAppUpdate(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)
	log := titancommon.Logger(ctx)

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

	app, err := Api.Datastore.StoreApp(wapp.App)
	if err != nil {
		log.WithError(err).Debug(models.ErrAppsCreate)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrAppsCreate))
		return
	}

	wapp.App = app

	// Nothing to update right now in apps
	c.JSON(http.StatusOK, appResponse{"App successfully updated", wapp.App})
}
