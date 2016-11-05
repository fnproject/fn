// Copyright 2016 Iron.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/runner"
	"github.com/iron-io/runner/common"
)

func handleRouteCreate(c *gin.Context) {
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
		newapp := &models.App{Name: wroute.Route.AppName}
		if err := newapp.Validate(); err != nil {
			log.Error(err)
			c.JSON(http.StatusInternalServerError, simpleError(err))
			return
		}

		app, err = Api.Datastore.StoreApp(newapp)
		if err != nil {
			log.WithError(err).Error(models.ErrAppsCreate)
			c.JSON(http.StatusInternalServerError, simpleError(models.ErrAppsCreate))
			return
		}
	}

	_, err = Api.Datastore.StoreRoute(wroute.Route)
	if err != nil {
		log.WithError(err).Error(models.ErrRoutesCreate)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrRoutesCreate))
		return
	}

	c.JSON(http.StatusCreated, routeResponse{"Route successfully created", wroute.Route})
}
