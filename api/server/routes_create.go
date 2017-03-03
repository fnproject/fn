package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/runner/common"
)

func (s *Server) handleRouteCreate(c *gin.Context) {
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
		log.WithError(err).Debug(models.ErrInvalidJSON)
		c.JSON(http.StatusBadRequest, simpleError(models.ErrRoutesMissingNew))
		return
	}

	wroute.Route.AppName = c.MustGet(api.AppName).(string)

	wroute.Route.SetDefaults()

	if err := wroute.Validate(false); err != nil {
		log.WithError(err).Debug(models.ErrRoutesCreate)
		c.JSON(http.StatusBadRequest, simpleError(err))
		return
	}

	// err = s.Runner.EnsureImageExists(ctx, &task.Config{
	// 	Image: wroute.Route.Image,
	// })
	// if err != nil {
	// 	c.JSON(http.StatusBadRequest, simpleError(models.ErrUsableImage))
	// 	return
	// }

	app, err := s.Datastore.GetApp(ctx, wroute.Route.AppName)
	if err != nil && err != models.ErrAppsNotFound {
		log.WithError(err).Error(models.ErrAppsGet)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrAppsGet))
		return
	} else if app == nil {
		// Create a new application and add the route to that new application
		newapp := &models.App{Name: wroute.Route.AppName}
		if err := newapp.Validate(); err != nil {
			log.Error(err)
			c.JSON(http.StatusInternalServerError, simpleError(err))
			return
		}

		err = s.FireBeforeAppCreate(ctx, newapp)
		if err != nil {
			log.WithError(err).Error(models.ErrAppsCreate)
			c.JSON(http.StatusInternalServerError, simpleError(ErrInternalServerError))
			return
		}

		_, err = s.Datastore.InsertApp(ctx, newapp)
		if err != nil {
			log.WithError(err).Error(models.ErrRoutesCreate)
			c.JSON(http.StatusInternalServerError, simpleError(ErrInternalServerError))
			return
		}

		err = s.FireAfterAppCreate(ctx, newapp)
		if err != nil {
			log.WithError(err).Error(models.ErrRoutesCreate)
			c.JSON(http.StatusInternalServerError, simpleError(ErrInternalServerError))
			return
		}

	}

	route, err := s.Datastore.InsertRoute(ctx, wroute.Route)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	s.cacherefresh(route)

	c.JSON(http.StatusOK, routeResponse{"Route successfully created", route})
}
