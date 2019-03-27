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
	"google.golang.org/grpc/status"
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
		log = log.WithField("root_error", w.RootError()).WithField("blame", w.Blame())
	}

	if ctx.Err() == context.Canceled {
		log.Info("client context cancelled")
		w.WriteHeader(models.ErrClientCancel.Code())
		return
	}

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
	} else if isGRPCError(err) {
		log.WithError(err).Info("gRPC error received")
		statuscode = http.StatusInternalServerError
		err = ErrInternalServerError
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

// isGRPCError inspect the error to verify if it is gRPC status and return a bool as results
// of this operation true means the error is a gRPC error false that it is not.
func isGRPCError(err error) bool {
	_, ok := status.FromError(err)
	return ok
}
