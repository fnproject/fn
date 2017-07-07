package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"gitlab-odx.oracle.com/odx/functions/api"
	"gitlab-odx.oracle.com/odx/functions/api/models"
)

func (s *Server) handleCallList(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)

	appName, ok := c.MustGet(api.AppName).(string)
	if ok && appName == "" {
		c.JSON(http.StatusBadRequest, models.ErrRoutesValidationMissingAppName)
		return
	}

	_, err := s.Datastore.GetApp(c, appName)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrAppsNotFound)
		return
	}

	appRoute, ok := c.MustGet(api.Path).(string)
	if ok && appRoute == "" {
		c.JSON(http.StatusBadRequest, models.ErrRoutesValidationMissingPath)
		return
	}
	_, err = s.Datastore.GetRoute(c, appName, appRoute)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrRoutesNotFound)
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
