package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"runtime/debug"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// ErrInternalServerError returned when something exceptional happens.
var ErrInternalServerError = errors.New("internal server error")

func simpleV1Error(err error) *models.ErrorWrapper {
	return &models.ErrorWrapper{Error: &models.Error{Message: err.Error()}}
}

func simpleError(err error) *models.Error {
	return &models.Error{Message: err.Error()}
}

// Legacy this is the old wrapped error
// TODO delete me !
func handleV1ErrorResponse(ctx *gin.Context, err error) {
	log := common.Logger(ctx)
	w := ctx.Writer
	var statuscode int
	if e, ok := err.(models.APIError); ok {
		if e.Code() >= 500 {
			log.WithFields(logrus.Fields{"code": e.Code()}).WithError(e).Error("api error")
		}
		if err == models.ErrCallTimeoutServerBusy {
			// TODO: Determine a better delay value here (perhaps ask Agent). For now 15 secs with
			// the hopes that fnlb will land this on a better server immediately.
			w.Header().Set("Retry-After", "15")
		}
		statuscode = e.Code()
	} else {
		log.WithError(err).WithFields(logrus.Fields{"stack": string(debug.Stack())}).Error("internal server error")
		statuscode = http.StatusInternalServerError
		err = ErrInternalServerError
	}
	writeV1Error(ctx, w, statuscode, err)
}

func handleErrorResponse(c *gin.Context, err error) {
	HandleErrorResponse(c.Request.Context(), c.Writer, err)
}

// WriteError easy way to do standard error response, but can set statuscode and error message easier than handleV1ErrorResponse
func writeV1Error(ctx context.Context, w http.ResponseWriter, statuscode int, err error) {
	log := common.Logger(ctx)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statuscode)
	err = json.NewEncoder(w).Encode(simpleV1Error(err))
	if err != nil {
		log.WithError(err).Errorln("error encoding error json")
	}
}

// handleV1ErrorResponse used to handle response errors in the same way.
func HandleErrorResponse(ctx context.Context, w http.ResponseWriter, err error) {
	log := common.Logger(ctx)
	var statuscode int
	if e, ok := err.(models.APIError); ok {
		if e.Code() >= 500 {
			log.WithFields(logrus.Fields{"code": e.Code()}).WithError(e).Error("api error")
		}
		if err == models.ErrCallTimeoutServerBusy {
			// TODO: Determine a better delay value here (perhaps ask Agent). For now 15 secs with
			// the hopes that fnlb will land this on a better server immediately.
			w.Header().Set("Retry-After", "15")
		}
		statuscode = e.Code()
	} else {
		log.WithError(err).WithFields(logrus.Fields{"stack": string(debug.Stack())}).Error("internal server error")
		statuscode = http.StatusInternalServerError
		err = ErrInternalServerError
	}
	WriteError(ctx, w, statuscode, err)
}

// WriteError easy way to do standard error response, but can set statuscode and error message easier than handleV1ErrorResponse
func WriteError(ctx context.Context, w http.ResponseWriter, statuscode int, err error) {
	log := common.Logger(ctx)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statuscode)
	err = json.NewEncoder(w).Encode(simpleError(err))
	if err != nil {
		log.WithError(err).Errorln("error encoding error json")
	}
}
