package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleShutdown(halt context.CancelFunc) func(*gin.Context) {
	return func(c *gin.Context) {
		halt()
		c.JSON(http.StatusOK, "shutting down")
	}
}
