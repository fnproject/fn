package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Version of IronFunctions
var Version = "0.1.35"

func handleVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"version": Version})
}
