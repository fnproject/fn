package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

// TODO: Deprecate with V1 API
func (s *Server) handleV1AppGetByName(c *gin.Context) {
	ctx := c.Request.Context()

	appID, err := s.datastore.GetAppID(ctx, c.Param(api.CApp))

	if err != nil {
		handleV1ErrorResponse(c, err)
		return
	}

	app, err := s.datastore.GetAppByID(ctx, appID)

	if err != nil {
		handleV1ErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, appResponse{"Successfully loaded app", app})
}
