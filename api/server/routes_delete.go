package server

import (
	"net/http"
	"path"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleRouteDelete(c *gin.Context) {
	ctx := c.Request.Context()

	appName := c.MustGet(api.AppName).(string)
	routePath := path.Clean(c.MustGet(api.Path).(string))

	if _, err := s.datastore.GetRoute(ctx, appName, routePath); err != nil {
		handleErrorResponse(c, err)
		return
	}

	if err := s.datastore.RemoveRoute(ctx, appName, routePath); err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Route deleted"})
}
