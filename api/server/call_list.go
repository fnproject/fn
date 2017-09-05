package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleCallList(c *gin.Context) {
	ctx := c.Request.Context()

	name, ok := c.Get(api.AppName)
	appName, conv := name.(string)
	if ok && conv && appName == "" {
		handleErrorResponse(c, models.ErrRoutesValidationMissingAppName)
		return
	}

	filter := models.CallFilter{AppName: appName, Path: c.Query(api.CRoute)}

	calls, err := s.Datastore.GetCalls(ctx, &filter)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	if len(calls) == 0 {
		_, err = s.Datastore.GetApp(c, appName)
		if err != nil {
			handleErrorResponse(c, err)
			return
		}

		if filter.Path != "" {
			_, err = s.Datastore.GetRoute(c, appName, filter.Path)
			if err != nil {
				handleErrorResponse(c, err)
				return
			}
		}
	}

	c.JSON(http.StatusOK, fnCallsResponse{"Successfully listed calls", calls})
}
