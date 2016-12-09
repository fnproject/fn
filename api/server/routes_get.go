package server

import (
	"context"
	"net/http"
	"path"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/runner/common"
)

func (s *Server) handleRouteGet(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)
	log := common.Logger(ctx)

	appName := ctx.Value("appName").(string)
	routePath := path.Clean(ctx.Value("routePath").(string))

	route, err := s.Datastore.GetRoute(ctx, appName, routePath)
	if err != nil && err != models.ErrRoutesNotFound {

		log.WithError(err).Error(models.ErrRoutesGet)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrRoutesGet))
		return
	} else if route == nil {
		c.JSON(http.StatusNotFound, simpleError(models.ErrRoutesNotFound))
		return
	}

	log.WithFields(logrus.Fields{"route": route}).Debug("Got route")

	c.JSON(http.StatusOK, routeResponse{"Successfully loaded route", route})
}
