package server

import (
	"encoding/base64"
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleRouteList(c *gin.Context) {
	ctx := c.Request.Context()

	var filter models.RouteFilter
	filter.Image = c.Query("image")
	// filter.PathPrefix = c.Query("path_prefix") TODO not hooked up
	filter.Cursor, filter.PerPage = pageParams(c, true)

	initApp := &models.App{Name: c.MustGet(api.App).(string)}
	app, err := s.datastore.GetApp(ctx, initApp)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}
	routes, err := s.datastore.GetRoutesByApp(ctx, app, &filter)

	var nextCursor string
	if len(routes) > 0 && len(routes) == filter.PerPage {
		last := []byte(routes[len(routes)-1].Path)
		nextCursor = base64.RawURLEncoding.EncodeToString(last)
	}

	c.JSON(http.StatusOK, routesResponse{
		Message:    "Successfully listed routes",
		NextCursor: nextCursor,
		Routes:     routes,
	})
}
