package server

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func handlePrometheus(c *gin.Context) {
	// TODO shouldn't instantiate this every request
	promhttp.Handler().ServeHTTP(c.Writer, c.Request)
}
