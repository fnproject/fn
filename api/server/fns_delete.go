package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleFnsDelete(c *gin.Context) {
	ctx := c.Request.Context()

	fn := c.Param(api.Fn)
	err := s.datastore.RemoveFn(ctx, fn)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Successfully deleted func"})
}
