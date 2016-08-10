package server

import (
	"net/http"

	"golang.org/x/net/context"

	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	titancommon "github.com/iron-io/titan/common"
)

func handleAppUpdate(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)
	log := titancommon.Logger(ctx)

	app := &models.App{}

	err := c.BindJSON(app)
	if err != nil {
		log.WithError(err).Debug(models.ErrInvalidJSON)
		c.JSON(http.StatusBadRequest, simpleError(models.ErrInvalidJSON))
		return
	}

	if app == nil {
		log.Debug(models.ErrAppsMissingNew)
		c.JSON(http.StatusBadRequest, simpleError(models.ErrAppsMissingNew))
		return
	}

	if err := app.Validate(); err != nil {
		log.Error(err)
		c.JSON(http.StatusInternalServerError, simpleError(err))
		return
	}

	// app, err := Api.Datastore.StoreApp(wapp.App)
	// if err != nil {
	// 	log.WithError(err).Debug(models.ErrAppsCreate)
	// 	c.JSON(http.StatusInternalServerError, simpleError(models.ErrAppsCreate))
	// 	return
	// }

	// Nothing to update right now in apps
	c.JSON(http.StatusOK, simpleError(models.ErrAppsNothingToUpdate))
}
