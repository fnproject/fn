package server

import (
	"errors"
	"net/http"
	"runtime/debug"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/runner/common"
)

// ErrInternalServerError returned when something exceptional happens.
var ErrInternalServerError = errors.New("internal server error")

func simpleError(err error) *models.Error {
	return &models.Error{Error: &models.ErrorBody{Message: err.Error()}}
}

func handleErrorResponse(c *gin.Context, err error) {
	log := common.Logger(c.Request.Context())
	switch e := err.(type) {
	case models.APIError:
		if e.Code() >= 500 {
			log.WithFields(logrus.Fields{"code": e.Code()}).WithError(e).Error("api error")
		}
		c.JSON(e.Code(), simpleError(e))
	default:
		log.WithError(err).WithFields(logrus.Fields{"stack": string(debug.Stack())}).Error("internal server error")
		c.JSON(http.StatusInternalServerError, simpleError(ErrInternalServerError))
	}
}
