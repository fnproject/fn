package router

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/runner"
)

func handleRunner(c *gin.Context) {
	log := c.MustGet("log").(logrus.FieldLogger)

	err := runner.Run(c)
	if err != nil {
		log.Debug(err)
		c.JSON(http.StatusInternalServerError, simpleError(err))
	}
}
