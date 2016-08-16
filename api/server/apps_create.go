package server

import (
	"net/http"

	"golang.org/x/net/context"

	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	titancommon "github.com/iron-io/titan/common"
)

func handleAppCreate(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)
	log := titancommon.Logger(ctx)

	var wapp models.AppWrapper

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

	if err := wapp.Validate(); err != nil {
		log.Error(err)
		c.JSON(http.StatusInternalServerError, simpleError(err))
		return
	}

	err = Api.FireBeforeAppUpdate(ctx, wapp.App)
	if err != nil {
		log.WithError(err).Errorln(models.ErrAppsCreate)
		c.JSON(http.StatusInternalServerError, simpleError(err))
		return
	}

	_, err = Api.Datastore.StoreApp(wapp.App)
	if err != nil {
		log.WithError(err).Errorln(models.ErrAppsCreate)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrAppsCreate))
		return
	}

	err = Api.FireAfterAppUpdate(ctx, wapp.App)
	if err != nil {
		log.WithError(err).Errorln(models.ErrAppsCreate)
		c.JSON(http.StatusInternalServerError, simpleError(err))
		return
	}

	c.JSON(http.StatusCreated, appResponse{"App  successfully created", wapp})
}
