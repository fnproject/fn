package server

import (
	"context"
	"net/http"
	"path"

	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/runner/common"
)

func (s *Server) handleRouteDelete(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)
	log := common.Logger(ctx)

	appName := c.MustGet(api.AppName).(string)
	routePath := path.Clean(c.MustGet(api.Path).(string))

	if err := s.Datastore.RemoveRoute(ctx, appName, routePath); err != nil {
		if err == models.ErrRoutesNotFound {
			log.WithError(err).Debug(models.ErrRoutesRemoving)
			c.JSON(http.StatusNotFound, simpleError(models.ErrRoutesNotFound))
		} else {
			log.WithError(err).Error(models.ErrRoutesRemoving)
			c.JSON(http.StatusInternalServerError, simpleError(models.ErrRoutesRemoving))
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Route deleted"})
}
