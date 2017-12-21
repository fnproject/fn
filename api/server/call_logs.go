package server

import (
	"bytes"
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleCallLogGet(c *gin.Context) {
	ctx := c.Request.Context()

	appName := c.MustGet(api.AppName).(string)
	callID := c.Param(api.Call)

	logReader, err := s.logstore.GetLog(ctx, appName, callID)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	// TODO this API needs to change to text/plain / gzip anyway, punting
	// optimization, but we can write this direct to the wire, too... seems like
	// we should write some kind of writev json thing for go since we keep
	// hitting this :(
	var b bytes.Buffer
	b.ReadFrom(logReader)

	callObj := models.CallLog{
		CallID:  callID,
		AppName: appName,
		Log:     b.String(),
	}

	c.JSON(http.StatusOK, callLogResponse{"Successfully loaded log", &callObj})
}
