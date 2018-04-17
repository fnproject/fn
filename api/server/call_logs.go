package server

import (
	"bytes"
	"io"
	"net/http"

	"errors"
	"strings"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

// note: for backward compatibility, will go away later
type callLogResponse struct {
	Message string   `json:"message"`
	Log     *CallLog `json:"log"`
}

type CallLog struct {
	CallID string `json:"call_id" db:"id"`
	Log    string `json:"log" db:"log"`
}

func writeJSON(c *gin.Context, callID string, logReader io.Reader) {
	var b bytes.Buffer
	b.ReadFrom(logReader)
	c.JSON(http.StatusOK, callLogResponse{"Successfully loaded log",
		&CallLog{
			CallID: callID,
			Log:    b.String(),
		}})
}

func (s *Server) handleCallLogGet(c *gin.Context) {
	ctx := c.Request.Context()

	appID := c.MustGet(api.AppID).(string)
	callID := c.Param(api.Call)

	logReader, err := s.logstore.GetLog(ctx, appID, callID)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	mimeTypes, _ := c.Request.Header["Accept"]

	if len(mimeTypes) == 0 {
		writeJSON(c, callID, logReader)
		return
	}

	for _, mimeType := range mimeTypes {
		if strings.Contains(mimeType, "application/json") {
			writeJSON(c, callID, logReader)
			return
		}
		if strings.Contains(mimeType, "text/plain") {
			io.Copy(c.Writer, logReader)
			return

		}
		if strings.Contains(mimeType, "*/*") {
			writeJSON(c, callID, logReader)
			return
		}
	}

	// if we've reached this point it means that Fn didn't recognize Accepted content type
	handleErrorResponse(c, models.NewAPIError(http.StatusNotAcceptable,
		errors.New("unable to respond within acceptable response content types")))
}
