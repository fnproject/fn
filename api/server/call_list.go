package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleCallList(c *gin.Context) {
	ctx := c.Request.Context()
	var err error

	fnID := c.Param(api.FnID)

	if fnID == "" {
		handleErrorResponse(c, models.ErrFnsMissingID)
		return
	}

	_, err = s.datastore.GetFnByID(ctx, c.Param(api.FnID))
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	filter := models.CallFilter{FnID: fnID}
	filter.Cursor, filter.PerPage = pageParams(c)

	filter.FromTime, filter.ToTime, err = timeParams(c)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	calls, err := s.logstore.GetCalls(ctx, &filter)

	if err != nil {
		handleErrorResponse(c, err)
	}

	c.JSON(http.StatusOK, calls)
}

// "" gets parsed to a zero time, which is fine (ignored in query)
func timeParams(c *gin.Context) (fromTime, toTime common.DateTime, err error) {
	fromStr := c.Query("from_time")
	toStr := c.Query("to_time")
	var ok bool
	if fromStr != "" {
		fromTime, ok = strToTime(fromStr)
		if !ok {
			return fromTime, toTime, models.ErrInvalidFromTime
		}
	}
	if toStr != "" {
		toTime, ok = strToTime(toStr)
		if !ok {
			return fromTime, toTime, models.ErrInvalidToTime
		}
	}
	return fromTime, toTime, nil
}

func strToTime(str string) (common.DateTime, bool) {
	sec, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return common.DateTime(time.Time{}), false
	}
	return common.DateTime(time.Unix(sec, 0)), true
}
