package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleFnsDelete(c *gin.Context) {
	ctx := c.Request.Context()

	fn := c.Param(api.Fn)
	appName := c.Param(api.CApp)

	appID, err := s.datastore.GetAppID(ctx, appName)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	err = s.datastore.RemoveFn(ctx, appID, fn)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Successfully deleted func"})
}
