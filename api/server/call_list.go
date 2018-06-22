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

	appID := c.MustGet(api.AppID).(string)
	// TODO api.CRoute needs to be escaped probably, since it has '/' a lot
	filter := models.CallFilter{AppID: appID, Path: c.Query("path")}
	filter.Cursor, filter.PerPage = pageParams(c, false) // ids are url safe

	filter.FromTime, filter.ToTime, err = timeParams(c)
	if err != nil {
		handleV1ErrorResponse(c, err)
		return
	}

	calls, err := s.logstore.GetCalls(ctx, &filter)

	var nextCursor string
	if len(calls) > 0 && len(calls) == filter.PerPage {
		nextCursor = calls[len(calls)-1].ID
		// don't base64, IDs are url safe
	}

	c.JSON(http.StatusOK, callsResponse{
		Message:    "Successfully listed calls",
		NextCursor: nextCursor,
		Calls:      calls,
	})
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
