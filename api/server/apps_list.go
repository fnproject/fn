package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/runner/common"
)

func handleAppList(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)
	log := common.Logger(ctx)

	filter := &models.AppFilter{}

	apps, err := Api.Datastore.GetApps(filter)
	if err != nil {
		log.WithError(err).Debug(models.ErrAppsList)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrAppsList))
		return
	}

	c.JSON(http.StatusOK, appsResponse{"Successfully listed applications", apps})
}
