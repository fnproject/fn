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
	var filter models.CallFilter
	var err error

	filter.Cursor, filter.PerPage = pageParamsV2(c)
	filter.AppID = c.Query(api.AppID)
	filter.FnID = c.Query("fn_id")

	filter.FromTime, filter.ToTime, err = timeParams(c)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	calls, err := s.logstore.GetCalls(ctx, &filter)

	// appCache := make(map[string]*models.App)

	// for idx, cl := range calls.Items {
	// 	app, ok := appCache[cl.AppID]
	// 	if !ok {
	// 		gotApp, err := s.Datastore().GetAppByID(ctx, cl.AppID)
	// 		if err != nil {
	// 			handleErrorResponse(c, fmt.Errorf("failed to get app from calls %s", err))
	// 			return
	// 		}
	// 		app = gotApp
	// 		appCache[app.ID] = gotApp
	// 	}

	// 	newC, err := s.callA

	// }
	// var nextCursor string
	// if len(calls) > 0 && len(calls) == filter.PerPage {
	// 	nextCursor = calls[len(calls)-1].ID
	// 	// don't base64, IDs are url safe
	// }

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
