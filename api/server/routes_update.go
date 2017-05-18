package server

import (
	"context"
	"net/http"
	"path"

	"github.com/gin-gonic/gin"
	"gitlab.oracledx.com/odx/functions/api"
	"gitlab.oracledx.com/odx/functions/api/models"
	"gitlab.oracledx.com/odx/functions/api/runner/common"
)

func (s *Server) handleRouteUpdate(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)
	log := common.Logger(ctx)

	var wroute models.RouteWrapper

	err := c.BindJSON(&wroute)
	if err != nil {
		log.WithError(err).Debug(models.ErrInvalidJSON)
		c.JSON(http.StatusBadRequest, simpleError(models.ErrInvalidJSON))
		return
	}

	if wroute.Route == nil {
		log.Debug(models.ErrRoutesMissingNew)
		c.JSON(http.StatusBadRequest, simpleError(models.ErrRoutesMissingNew))
		return
	}

	if wroute.Route.Path != "" {
		log.Debug(models.ErrRoutesPathImmutable)
		c.JSON(http.StatusBadRequest, simpleError(models.ErrRoutesPathImmutable))
		return
	}

	wroute.Route.AppName = c.MustGet(api.AppName).(string)
	wroute.Route.Path = path.Clean(c.MustGet(api.Path).(string))

	if wroute.Route.IdleTimeout == 0 {
		wroute.Route.IdleTimeout = 30
	}

	if err := wroute.Validate(true); err != nil {
		log.WithError(err).Debug(models.ErrRoutesUpdate)
		c.JSON(http.StatusBadRequest, simpleError(err))
		return
	}

	if wroute.Route.Image != "" {
		// This was checking that an image exists, but it's too slow of an operation. Checks at runtime now.
		// err = s.Runner.EnsureImageExists(ctx, &task.Config{
		// 	Image: wroute.Route.Image,
		// })
		// if err != nil {
		// 	log.WithError(err).Debug(models.ErrRoutesUpdate)
		// 	c.JSON(http.StatusBadRequest, simpleError(models.ErrUsableImage))
		// 	return
		// }
	}

	route, err := s.Datastore.UpdateRoute(ctx, wroute.Route)
	if err == models.ErrRoutesNotFound {
		// try insert then
		route, err = s.Datastore.InsertRoute(ctx, wroute.Route)
	}
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	s.cacherefresh(route)

	c.JSON(http.StatusOK, routeResponse{"Route successfully updated", route})
}
