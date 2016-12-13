package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/runner/common"
)

func (s *Server) handleAppList(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)
	log := common.Logger(ctx)

	filter := &models.AppFilter{}

	apps, err := s.Datastore.GetApps(ctx, filter)
	if err != nil {
		log.WithError(err).Error(models.ErrAppsList)
		c.JSON(http.StatusInternalServerError, simpleError(ErrInternalServerError))
		return
	}

	c.JSON(http.StatusOK, appsResponse{"Successfully listed applications", apps})
}
