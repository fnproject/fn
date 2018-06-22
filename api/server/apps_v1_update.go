package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

// TODO: Deprecate with V1 API
func (s *Server) handleV1AppUpdate(c *gin.Context) {
	ctx := c.Request.Context()

	wapp := models.AppWrapper{}

	err := c.BindJSON(&wapp)
	if err != nil {
		if models.IsAPIError(err) {
			handleV1ErrorResponse(c, err)
		} else {
			handleV1ErrorResponse(c, models.ErrInvalidJSON)
		}
		return
	}

	if wapp.App == nil {
		handleV1ErrorResponse(c, models.ErrAppsMissingNew)
		return
	}

	if wapp.App.Name != "" {
		handleV1ErrorResponse(c, models.ErrAppsNameImmutable)
		return
	}

	wapp.App.Name = c.MustGet(api.App).(string)
	wapp.App.ID = c.MustGet(api.AppID).(string)

	app, err := s.datastore.UpdateApp(ctx, wapp.App)
	if err != nil {
		handleV1ErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, appResponse{"App successfully updated", app})
}
