package server

import (
	"context"
	"net/http"
	"path"

	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/runner/common"
)

func (s *Server) handleRouteDelete(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)
	log := common.Logger(ctx)

	appName := ctx.Value("appName").(string)
	routePath := path.Clean(ctx.Value("routePath").(string))

	route, err := s.Datastore.GetRoute(ctx, appName, routePath)
	if err != nil || route == nil {
		log.Error(models.ErrRoutesNotFound)
		c.JSON(http.StatusNotFound, simpleError(models.ErrRoutesNotFound))
		return
	}

	if err := s.Datastore.RemoveRoute(ctx, appName, routePath); err != nil {
		log.WithError(err).Debug(models.ErrRoutesRemoving)
		if err == models.ErrRoutesNotFound {
			c.JSON(http.StatusNotFound, simpleError(models.ErrRoutesNotFound))
		} else {
			c.JSON(http.StatusInternalServerError, simpleError(models.ErrRoutesRemoving))
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Route deleted"})
}
