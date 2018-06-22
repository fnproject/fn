package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

// TODO: Deprecate with v1
func (s *Server) handleV1AppDelete(c *gin.Context) {
	ctx := c.Request.Context()

	err := s.datastore.RemoveApp(ctx, c.MustGet(api.AppID).(string))
	if err != nil {
		handleV1ErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "App deleted"})
}
