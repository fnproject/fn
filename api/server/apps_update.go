package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gitlab-odx.oracle.com/odx/functions/api"
	"gitlab-odx.oracle.com/odx/functions/api/models"
)

func (s *Server) handleAppUpdate(c *gin.Context) {
	ctx := c.MustGet("mctx").(MiddlewareContext)

	wapp := models.AppWrapper{}

	err := c.BindJSON(&wapp)
	if err != nil {
		handleErrorResponse(c, models.ErrInvalidJSON)
		return
	}

	if wapp.App == nil {
		handleErrorResponse(c, models.ErrAppsMissingNew)
		return
	}

	if wapp.App.Name != "" {
		handleErrorResponse(c, models.ErrAppsNameImmutable)
		return
	}

	wapp.App.Name = c.MustGet(api.AppName).(string)

	err = s.FireAfterAppUpdate(ctx, wapp.App)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	app, err := s.Datastore.UpdateApp(ctx, wapp.App)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	err = s.FireAfterAppUpdate(ctx, wapp.App)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, appResponse{"App successfully updated", app})
}
