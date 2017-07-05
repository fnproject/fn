package server

import (
	"context"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"gitlab-odx.oracle.com/odx/functions/api"
	"gitlab-odx.oracle.com/odx/functions/api/models"
	"gitlab-odx.oracle.com/odx/functions/api/runner/common"
)

/* handleRouteCreateOrUpdate is used to handle POST PUT and PATCH for routes.
   Post will only create route if its not there and create app if its not.
       create only
   Put will create app if its not there and if route is there update if not it will create new route.
       update if exists or create if not exists
   Patch will not create app if it does not exist since the route needs to exist as well...
       update only
*/
func (s *Server) handleRouteCreateOrUpdate(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)
	log := common.Logger(ctx)
	method := strings.ToUpper(c.Request.Method)

	var wroute models.RouteWrapper

	err := c.BindJSON(&wroute)
	if err != nil {
		log.WithError(err).Debug(models.ErrInvalidJSON)
		c.JSON(http.StatusBadRequest, simpleError(models.ErrInvalidJSON))
		return
	}

	if wroute.Route == nil {
		log.WithError(err).Debug(models.ErrRoutesMissingNew)
		c.JSON(http.StatusBadRequest, simpleError(models.ErrRoutesMissingNew))
		return
	}

	wroute.Route.AppName = c.MustGet(api.AppName).(string)

	if method == http.MethodPut || method == http.MethodPatch {
		p := path.Clean(c.MustGet(api.Path).(string))

		if wroute.Route.Path != "" && wroute.Route.Path != p {
			log.Debug(models.ErrRoutesPathImmutable)
			c.JSON(http.StatusBadRequest, simpleError(models.ErrRoutesPathImmutable))
			return
		}
		wroute.Route.Path = p
	}

	wroute.Route.SetDefaults()

	if err = wroute.Validate(method == http.MethodPost); err != nil {
		log.WithError(err).Debug(models.ErrRoutesCreate)
		c.JSON(http.StatusBadRequest, simpleError(err))
		return
	}

	//Create the app if it does not exist.
	if method == http.MethodPost || method == http.MethodPut {
		var app *models.App
		app, err = s.Datastore.GetApp(ctx, wroute.Route.AppName)
		if err != nil && err != models.ErrAppsNotFound {
			log.WithError(err).Error(models.ErrAppsGet)
			c.JSON(http.StatusInternalServerError, simpleError(models.ErrAppsGet))
			return
		} else if app == nil {
			// Create a new application and add the route to that new application
			newapp := &models.App{Name: wroute.Route.AppName}
			if err = newapp.Validate(); err != nil {
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
	}

	var route *models.Route

	resp := routeResponse{"Route successfully created", route}
	up := routeResponse{"Route successfully updated", route}

	switch method {
	case http.MethodPost:
		route, err = s.Datastore.InsertRoute(ctx, wroute.Route)
	case http.MethodPut:
		route, err = s.Datastore.UpdateRoute(ctx, wroute.Route)
		if err == models.ErrRoutesNotFound {
			// try insert then
			route, err = s.Datastore.InsertRoute(ctx, wroute.Route)
		}
	case http.MethodPatch:
		route, err = s.Datastore.UpdateRoute(ctx, wroute.Route)
		resp = up
	}

	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	s.cacheRefresh(route)

	c.JSON(http.StatusOK, resp)
}
