package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/models"
)

func (s *Server) handleCallList(c *gin.Context) {
	ctx := c.Request.Context()

	appName, ok := c.MustGet(api.AppName).(string)
	if ok && appName == "" {
		handleErrorResponse(c, models.ErrRoutesValidationMissingAppName)
		return
	}

	_, err := s.Datastore.GetApp(c, appName)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	appRoute, ok := c.MustGet(api.Path).(string)
	if ok && appRoute == "" {
		handleErrorResponse(c, models.ErrRoutesValidationMissingPath)
		return
	}
	_, err = s.Datastore.GetRoute(c, appName, appRoute)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	filter := models.CallFilter{AppName: appName, Path: appRoute}

	calls, err := s.Datastore.GetTasks(ctx, &filter)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, fnCallsResponse{"Successfully listed calls", calls})
}
