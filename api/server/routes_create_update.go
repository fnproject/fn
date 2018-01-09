package server

import (
	"context"
	"net/http"
	"path"
	"strings"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

/* handleRouteCreateOrUpdate is used to handle POST PUT and PATCH for routes.
   Post will only create route if its not there and create app if its not.
       create only
	   Post does not skip validation of zero values
   Put will create app if its not there and if route is there update if not it will create new route.
       update if exists or create if not exists
	   Put does not skip validation of zero values
   Patch will not create app if it does not exist since the route needs to exist as well...
       update only
	   Patch accepts partial updates / skips validation of zero values.
*/
func (s *Server) handleRoutesPostPutPatch(c *gin.Context) {
	ctx := c.Request.Context()
	method := strings.ToUpper(c.Request.Method)

	var wroute models.RouteWrapper
	err := bindRoute(c, method, &wroute)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}
	if method != http.MethodPatch {
		err = s.ensureApp(ctx, &wroute, method)
		if err != nil {
			handleErrorResponse(c, err)
			return
		}
	}
	resp, err := s.ensureRoute(ctx, method, &wroute)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) submitRoute(ctx context.Context, wroute *models.RouteWrapper) error {
	r, err := s.datastore.InsertRoute(ctx, wroute.Route)
	if err != nil {
		return err
	}
	wroute.Route = r
	return nil
}

func (s *Server) changeRoute(ctx context.Context, wroute *models.RouteWrapper) error {
	r, err := s.datastore.UpdateRoute(ctx, wroute.Route)
	if err != nil {
		return err
	}
	wroute.Route = r
	return nil
}

// ensureApp will only execute if it is everything except PATCH
func (s *Server) ensureRoute(ctx context.Context, method string, wroute *models.RouteWrapper) (routeResponse, error) {
	bad := new(routeResponse)

	initApp := &models.App{Name: wroute.Route.AppID}
	app, err := s.Datastore().GetApp(ctx, initApp)
	if err != nil {
		return *bad, err
	}
	wroute.Route.AppID = app.ID

	switch method {
	case http.MethodPost:
		err := s.submitRoute(ctx, wroute)
		if err != nil {
			return *bad, err
		}
		return routeResponse{"Route successfully created", wroute.Route}, nil
	case http.MethodPut:
		_, err := s.datastore.GetRoute(ctx, app, wroute.Route.Path)
		if err != nil && err == models.ErrRoutesNotFound {
			err := s.submitRoute(ctx, wroute)
			if err != nil {
				return *bad, err
			}
			return routeResponse{"Route successfully created", wroute.Route}, nil
		}
		err = s.changeRoute(ctx, wroute)
		if err != nil {
			return *bad, err
		}
		return routeResponse{"Route successfully updated", wroute.Route}, nil
	case http.MethodPatch:
		err := s.changeRoute(ctx, wroute)
		if err != nil {
			return *bad, err
		}
		return routeResponse{"Route successfully updated", wroute.Route}, nil
	}
	return *bad, nil
}

// ensureApp will only execute if it is on post or put. Patch is not allowed to create apps.
func (s *Server) ensureApp(ctx context.Context, wroute *models.RouteWrapper, method string) error {
	initApp := &models.App{Name: wroute.Route.AppID}
	app, err := s.datastore.GetApp(ctx, initApp)
	if err != nil && err != models.ErrAppsNotFound {
		return err
	} else if app == nil {
		// Create a new application
		newapp := &models.App{Name: wroute.Route.AppName}
		_, err = s.datastore.InsertApp(ctx, newapp)
		if err != nil {
			return err
		}
	}
	return nil
}

// bindRoute binds the RouteWrapper to the json from the request.
func bindRoute(c *gin.Context, method string, wroute *models.RouteWrapper) error {
	err := c.BindJSON(wroute)
	if err != nil {
		if models.IsAPIError(err) {
			return err
		}
		return models.ErrInvalidJSON
	}

	if wroute.Route == nil {
		return models.ErrRoutesMissingNew
	}

	wroute.Route.AppID = c.MustGet(api.App).(string)

	if method == http.MethodPut || method == http.MethodPatch {
		p := path.Clean(c.MustGet(api.Path).(string))

		if wroute.Route.Path != "" && wroute.Route.Path != p {
			return models.ErrRoutesPathImmutable
		}
		wroute.Route.Path = p
	}
	if method == http.MethodPost {
		if wroute.Route.Path == "" {
			return models.ErrRoutesMissingPath
		}
	}
	return nil
}
