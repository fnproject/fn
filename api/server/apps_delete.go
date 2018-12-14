package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleAppDelete(c *gin.Context) {
	ctx := c.Request.Context()

	err := s.datastore.RemoveApp(ctx, c.Param(api.AppID))
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.String(http.StatusNoContent, "")
}
