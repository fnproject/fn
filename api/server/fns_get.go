package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleFnGet(c *gin.Context) {
	ctx := c.Request.Context()
	f, err := s.datastore.GetFnByID(ctx, c.Param(api.FnID))

	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, f)
}
