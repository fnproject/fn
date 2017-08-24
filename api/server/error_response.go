package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"runtime/debug"

	"github.com/sirupsen/logrus"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/runner/common"
	"github.com/gin-gonic/gin"
)

// ErrInternalServerError returned when something exceptional happens.
var ErrInternalServerError = errors.New("internal server error")

func simpleError(err error) *models.Error {
	return &models.Error{Error: &models.ErrorBody{Message: err.Error()}}
}

func handleErrorResponse(c *gin.Context, err error) {
	HandleErrorResponse(c.Request.Context(), c.Writer, err)
}

// HandleErrorResponse used to handle response errors in the same way.
func HandleErrorResponse(ctx context.Context, w http.ResponseWriter, err error) {
	log := common.Logger(ctx)
	var statuscode int
	switch e := err.(type) {
	case models.APIError:
		if e.Code() >= 500 {
			log.WithFields(logrus.Fields{"code": e.Code()}).WithError(e).Error("api error")
		}
		statuscode = e.Code()
	default:
		log.WithError(err).WithFields(logrus.Fields{"stack": string(debug.Stack())}).Error("internal server error")
		statuscode = http.StatusInternalServerError
		err = ErrInternalServerError
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statuscode)
	err = json.NewEncoder(w).Encode(simpleError(err))
	if err != nil {
		log.WithError(err).Errorln("error encoding error json")
	}
}
