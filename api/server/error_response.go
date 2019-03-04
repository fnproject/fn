package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"runtime/debug"
	"strconv"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// ErrInternalServerError returned when something exceptional happens.
var ErrInternalServerError = errors.New("internal server error")

func simpleError(err error) *models.Error {
	return &models.Error{Message: err.Error()}
}

func handleErrorResponse(c *gin.Context, err error) {
	HandleErrorResponse(c.Request.Context(), c.Writer, err)
}

// HandleErrorResponse used to handle response errors in the same way.
func HandleErrorResponse(ctx context.Context, w http.ResponseWriter, err error) {
	log := common.Logger(ctx)
	if w, ok := err.(models.APIErrorWrapper); ok {
		log = log.WithField("root_error", w.RootError())
	}

	if ctx.Err() == context.Canceled {
		log.Info("client context cancelled")
		w.WriteHeader(models.ErrClientCancel.Code())
		return
	}

	var statuscode int

	switch e := err.(type) {
	case models.APIError:
		code := e.Code()
		if code >= 500 {
			log.WithFields(logrus.Fields{"code": code}).WithError(e).Error("api error")
		}
		statuscode = code
	case models.RetryableError:
		statuscode = e.APIErrorWrapper().Code()
		w.Header().Set("Retry-After", strconv.Itoa(e.RetryAfter()))
		// Set the generic error for retry
		err = e.APIErrorWrapper()
		// Log the root cause, this error doesn't go back to the client
		log.WithError(e.APIErrorWrapper().RootError()).Error("api retrayable error")
	default:
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
