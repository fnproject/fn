package server

import (
	"fmt"
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

	app, err := s.datastore.GetAppByID(ctx, f.AppID)
	if err != nil {
		handleErrorResponse(c, fmt.Errorf("unexpected error - fn app not available: %s", err))
		return
	}

	f, err = s.fnAnnotator.AnnotateFn(c, app, f)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, f)
}
