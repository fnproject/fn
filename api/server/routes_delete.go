package server

import (
	"net/http"
	"path"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleRouteDelete(c *gin.Context) {
	ctx := c.Request.Context()

	appID := c.MustGet(api.AppID).(string)
	routePath := path.Clean(c.MustGet(api.Path).(string))

	if _, err := s.datastore.GetRoute(ctx, appID, routePath); err != nil {
		handleErrorResponse(c, err)
		return
	}

	if err := s.datastore.RemoveRoute(ctx, appID, routePath); err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Route deleted"})
}
