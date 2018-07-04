package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

// TODO: Deprecate with V1 API
func (s *Server) handleV1AppGetByIdOrName(c *gin.Context) {
	ctx := c.Request.Context()

	param := c.MustGet(api.AppID).(string)

	app, err := s.datastore.GetAppByID(ctx, param)

	if err != nil {
		handleV1ErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, appResponse{"Successfully loaded app", app})
}
