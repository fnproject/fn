package server

import (
	"bytes"
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

// note: for backward compatibility, will go away later
type callLogResponse struct {
	Message string          `json:"message"`
	Log     *models.CallLog `json:"log"`
}

func (s *Server) handleCallLogGet(c *gin.Context) {
	ctx := c.Request.Context()

	appName := c.MustGet(api.AppName).(string)
	callID := c.Param(api.Call)

	logReader, err := s.logstore.GetLog(ctx, appName, callID)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	var b bytes.Buffer
	b.ReadFrom(logReader)

	switch c.Request.Header.Get("Accept") {
	case "application/json":
		callObj := models.CallLog{
			CallID:  callID,
			AppName: appName,
			Log:     b.String(),
		}

		c.JSON(http.StatusOK, callLogResponse{"Successfully loaded log", &callObj})
	default:
		c.String(http.StatusOK, b.String())
	}
}
