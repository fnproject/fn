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

	if wapp.App == nil {
		handleErrorResponse(c, models.ErrAppsMissingNew)
		return
	}

	if err = wapp.Validate(); err != nil {
		handleErrorResponse(c, err)
		return
	}

	err = s.FireBeforeAppCreate(ctx, wapp.App)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	app, err := s.Datastore.InsertApp(ctx, wapp.App)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	err = s.FireAfterAppCreate(ctx, wapp.App)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, appResponse{"App successfully created", app})
}
