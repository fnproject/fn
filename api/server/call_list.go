package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
	"github.com/go-openapi/strfmt"
)

func (s *Server) handleCallList(c *gin.Context) {
	ctx := c.Request.Context()

	appIDorName := c.MustGet(api.App).(string)

	// TODO api.CRoute needs to be escaped probably, since it has '/' a lot
	filter := models.CallFilter{AppName: appIDorName, Path: c.Query("path"), AppID: appIDorName}
	filter.Cursor, filter.PerPage = pageParams(c, false) // ids are url safe

	var err error
	filter.FromTime, filter.ToTime, err = timeParams(c)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	calls, err := s.datastore.GetCalls(ctx, &filter)

	if len(calls) == 0 {
		// TODO this should be done in front of this handler to even get here...
		_, err = s.datastore.GetApp(c, appIDorName)
	}

	if err != nil {
		handleErrorResponse(c, err)
		return
	}

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
func timeParams(c *gin.Context) (fromTime, toTime strfmt.DateTime, err error) {
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

func strToTime(str string) (strfmt.DateTime, bool) {
	sec, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return strfmt.DateTime(time.Time{}), false
	}
	return strfmt.DateTime(time.Unix(sec, 0)), true
}
