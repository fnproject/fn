package server

import (
	"net/http"

	"golang.org/x/net/context"

	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	titancommon "github.com/iron-io/titan/common"
)

func handleAppList(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)
	log := titancommon.Logger(ctx)

	filter := &models.AppFilter{}

	apps, err := Api.Datastore.GetApps(filter)
	if err != nil {
		log.WithError(err).Debug(models.ErrAppsList)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrAppsList))
		return
	}

	c.JSON(http.StatusOK, &models.AppsWrapper{apps})
}
