package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func handleVersion(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, "Not Implemented")
}
