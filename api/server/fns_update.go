package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleFnUpdate(c *gin.Context) {
	ctx := c.Request.Context()

	fn := &models.Fn{}
	err := c.BindJSON(fn)
	if err != nil {
		if !models.IsAPIError(err) {
			err = models.ErrInvalidJSON
		}
		handleErrorResponse(c, err)
		return
	}

	pathFnID := c.Param(api.FnID)

	if fn.ID == "" {
		fn.ID = pathFnID
	} else {
		if pathFnID != fn.ID {
			handleErrorResponse(c, models.ErrFnsIDMismatch)
		}
	}

	fnUpdated, err := s.datastore.UpdateFn(ctx, fn)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, fnUpdated)
}
