package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
)

func (s *Server) handleAppList(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)

	filter := &models.AppFilter{}

	apps, err := s.Datastore.GetApps(ctx, filter)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, appsResponse{"Successfully listed applications", apps})
}
