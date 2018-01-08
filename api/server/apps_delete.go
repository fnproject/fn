package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleAppDelete(c *gin.Context) {
	ctx := c.Request.Context()

	appName := c.MustGet(api.App).(string)
	err := s.datastore.RemoveApp(ctx, appName)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "App deleted"})
}
