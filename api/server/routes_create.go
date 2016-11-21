package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/runner"
	"github.com/iron-io/runner/common"
)

func (s *Server) handleRouteCreate(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)
	log := common.Logger(ctx)

	var wroute models.RouteWrapper

	err := c.BindJSON(&wroute)
	if err != nil {
		log.WithError(err).Error(models.ErrInvalidJSON)
		c.JSON(http.StatusBadRequest, simpleError(models.ErrInvalidJSON))
		return
	}

	if wroute.Route == nil {
		log.WithError(err).Error(models.ErrInvalidJSON)
		c.JSON(http.StatusBadRequest, simpleError(models.ErrRoutesMissingNew))
		return
	}

	wroute.Route.AppName = c.Param("app")

	if err := wroute.Validate(); err != nil {
		log.Error(err)
		c.JSON(http.StatusInternalServerError, simpleError(err))
		return
	}

	if wroute.Route.Image == "" {
		c.JSON(http.StatusBadRequest, simpleError(models.ErrRoutesValidationMissingImage))
		return
	}
	err = Api.Runner.EnsureImageExists(ctx, &runner.Config{
		Image: wroute.Route.Image,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrUsableImage))
		return
	}

	app, err := Api.Datastore.GetApp(wroute.Route.AppName)
	if err != nil {
		log.WithError(err).Error(models.ErrAppsGet)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrAppsGet))
		return
	}

	if app == nil {
		// Create a new application and add the route to that new application
		newapp := &models.App{Name: wroute.Route.AppName}
		if err := newapp.Validate(); err != nil {
			log.Error(err)
			c.JSON(http.StatusInternalServerError, simpleError(err))
			return
		}

		app, err = Api.Datastore.InsertApp(newapp)
		if err != nil {
			log.WithError(err).Error(models.ErrAppsCreate)
			c.JSON(http.StatusInternalServerError, simpleError(models.ErrAppsCreate))
			return
		}
	}

	_, err = Api.Datastore.InsertRoute(wroute.Route)
	if err != nil {
		log.WithError(err).Error(models.ErrRoutesCreate)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrRoutesCreate))
		return
	}

	s.resetcache(wroute.Route.AppName, 1)

	c.JSON(http.StatusCreated, routeResponse{"Route successfully created", wroute.Route})
}
