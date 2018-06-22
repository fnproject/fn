package server

import (
	"net/http"

	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleFnCreate(c *gin.Context) {
	ctx := c.Request.Context()
	fn := &models.Fn{}
	err := c.BindJSON(&fn)
	if err != nil {
		if !models.IsAPIError(err) {
			err = models.ErrInvalidJSON
		}
		handleErrorResponse(c, err)
		return
	}

	fn, err = s.datastore.InsertFn(ctx, fn)

	if err != nil {
		handleErrorResponse(c, err)
		return

	}
	c.JSON(http.StatusOK, fn)
}
