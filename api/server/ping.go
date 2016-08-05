package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func handlePing(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, "Not Implemented")
}
