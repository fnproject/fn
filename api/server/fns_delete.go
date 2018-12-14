package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleFnDelete(c *gin.Context) {
	ctx := c.Request.Context()

	fnID := c.Param(api.FnID)

	err := s.datastore.RemoveFn(ctx, fnID)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.String(http.StatusNoContent, "")
}
