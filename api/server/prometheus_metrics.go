package server

import (
	"github.com/gin-gonic/gin"
)

func (s *Server) handlePrometheusMetrics(c *gin.Context) {
	s.Agent.PromHandler().ServeHTTP(c.Writer, c.Request)
}
