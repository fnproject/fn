package server

import (
	"context"
	"errors"
	"net/http"
	"runtime/debug"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"gitlab-odx.oracle.com/odx/functions/api/models"
	"gitlab-odx.oracle.com/odx/functions/api/runner/common"
)

var ErrInternalServerError = errors.New("internal server error")

func simpleError(err error) *models.Error {
	return &models.Error{Error: &models.ErrorBody{Message: err.Error()}}
}

func handleErrorResponse(c *gin.Context, err error) {
	ctx := c.MustGet("ctx").(context.Context)
	log := common.Logger(ctx)

	if aerr, ok := err.(models.APIError); ok {
		log.WithFields(logrus.Fields{"code": aerr.Code()}).WithError(err).Error("api error")
		c.JSON(aerr.Code(), simpleError(err))
	} else if err != nil {
		// get a stack trace so we can trace this error
		log.WithError(err).WithFields(logrus.Fields{"stack": string(debug.Stack())}).Error("internal server error")
		c.JSON(http.StatusInternalServerError, simpleError(ErrInternalServerError))
	}
}
