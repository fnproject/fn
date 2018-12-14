package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleAppUpdate(c *gin.Context) {
	ctx := c.Request.Context()

	app := &models.App{}

	err := c.BindJSON(app)
	if err != nil {
		if models.IsAPIError(err) {
			handleErrorResponse(c, err)
		} else {
			handleErrorResponse(c, models.ErrInvalidJSON)
		}
		return
	}

	id := c.Param(api.AppID)

	if app.ID == "" {
		app.ID = id
	}
	if app.ID != id {
		handleErrorResponse(c, models.ErrAppsIDMismatch)
		return
	}
	app, err = s.datastore.UpdateApp(ctx, app)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, app)
}
