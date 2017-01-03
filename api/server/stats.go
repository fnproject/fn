package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleStats(c *gin.Context) {
	c.JSON(http.StatusOK, s.Runner.Stats())
}
