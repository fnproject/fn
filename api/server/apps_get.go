package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gitlab-odx.oracle.com/odx/functions/api"
)

func (s *Server) handleAppGet(c *gin.Context) {
	ctx := c.Request.Context()

	appName := c.MustGet(api.AppName).(string)
	app, err := s.Datastore.GetApp(ctx, appName)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, appResponse{"Successfully loaded app", app})
}
