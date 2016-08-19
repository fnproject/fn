package server

import (
	"net/http"

	"golang.org/x/net/context"

	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	titancommon "github.com/iron-io/titan/common"
)

func handleAppDelete(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)
	log := titancommon.Logger(ctx)

	appName := c.Param("app")
	err := Api.Datastore.RemoveApp(appName)

	if err != nil {
		log.WithError(err).Debug(models.ErrAppsRemoving)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrAppsRemoving))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "App deleted"})
}
