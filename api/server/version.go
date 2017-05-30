package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gitlab-odx.oracle.com/odx/functions/api/version"
)

func handleVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"version": version.Version})
}
