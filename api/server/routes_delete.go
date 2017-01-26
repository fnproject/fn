package server

import (
	"context"
	"net/http"
	"path"

	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api"
)

func (s *Server) handleRouteDelete(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)

	appName := c.MustGet(api.AppName).(string)
	routePath := path.Clean(c.MustGet(api.Path).(string))

	if err := s.Datastore.RemoveRoute(ctx, appName, routePath); err != nil {
		handleErrorResponse(c, err)
		return
	}

	s.cachedelete(appName, routePath)
	c.JSON(http.StatusOK, gin.H{"message": "Route deleted"})
}
