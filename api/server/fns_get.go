package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleFnsGet(c *gin.Context) {
	ctx := c.Request.Context()

	fn := c.Param(api.Fn)
	appName := c.Param(api.CApp)

	appID, err := s.datastore.GetAppID(ctx, appName)

	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	f, err := s.datastore.GetFn(ctx, appID, fn)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, fnResponse{"Successfully loaded func", f})
}
