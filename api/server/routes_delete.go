package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/runner/common"
)

func (s *Server) handleRouteDelete(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)
	log := common.Logger(ctx)

	appName := c.Param("app")
	routePath := c.Param("route")
	err := Api.Datastore.RemoveRoute(appName, routePath)

	if err != nil {
		log.WithError(err).Debug(models.ErrRoutesRemoving)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrRoutesRemoving))
		return
	}

	s.resetcache(appName, 0)

	c.JSON(http.StatusOK, gin.H{"message": "Route deleted"})
}
