package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func handlePing(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"hello": "world!", "goto": "https://github.com/fnproject/fn"})
}
