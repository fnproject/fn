package server

import (
	"fmt"
	"net/http"

	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleFnCreate(c *gin.Context) {
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

	fn.SetDefaults()
	fnCreated, err := s.datastore.InsertFn(ctx, fn)
	if err != nil {
		handleErrorResponse(c, err)
	}

	app, err := s.datastore.GetAppByID(ctx, fnCreated.AppID)
	if err != nil {
		handleErrorResponse(c, fmt.Errorf("unexpected error - fn app not available: %s", err))
		return
	}

	fnCreated, err = s.fnAnnotator.AnnotateFn(c, app, fnCreated)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, fnCreated)
}
