package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleAppGet(c *gin.Context) {
	ctx := c.Request.Context()

	appName := c.MustGet(api.AppName).(string)

	err := s.FireBeforeAppGet(ctx, appName)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	app, err := s.Datastore().GetApp(ctx, appName)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}
	err = s.FireAfterAppGet(ctx, app)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, appResponse{"Successfully loaded app", app})
}
