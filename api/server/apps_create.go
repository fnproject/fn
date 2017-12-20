package server

import (
	"net/http"

	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleAppCreate(c *gin.Context) {
	ctx := c.Request.Context()

	var wapp models.AppWrapper

	err := c.BindJSON(&wapp)
	if err != nil {
		handleErrorResponse(c, models.ErrInvalidJSON)
		return
	}

	app := wapp.App
	if app == nil {
		handleErrorResponse(c, models.ErrAppsMissingNew)
		return
	}

	app.SetDefaults()
	if err = app.Validate(); err != nil {
		handleErrorResponse(c, err)
		return
	}

	err = s.FireBeforeAppCreate(ctx, app)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	app, err := s.datastore.InsertApp(ctx, app)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	err = s.FireAfterAppCreate(ctx, app)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, appResponse{"App successfully created", app})
}
