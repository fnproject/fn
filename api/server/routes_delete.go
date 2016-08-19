package server

import (
	"net/http"

	"golang.org/x/net/context"

	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	titancommon "github.com/iron-io/titan/common"
)

func handleRouteDelete(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)
	log := titancommon.Logger(ctx)

	appName := c.Param("app")
	routeName := c.Param("route")
	err := Api.Datastore.RemoveRoute(appName, routeName)

	if err != nil {
		log.WithError(err).Debug(models.ErrRoutesRemoving)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrRoutesRemoving))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Route deleted"})
}
