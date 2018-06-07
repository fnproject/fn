package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleFnsPut(c *gin.Context) {
	ctx := c.Request.Context()

	var wfn models.FnWrapper
	err := c.BindJSON(&wfn)
	if err != nil {
		if !models.IsAPIError(err) {
			// TODO this error message sucks
			err = models.ErrInvalidJSON
		}
		handleErrorResponse(c, err)
		return
	}
	if wfn.Fn == nil {
		handleErrorResponse(c, models.ErrFnsMissingNew)
		return
	}

	fn := c.Param(api.Fn)
	// TODO: what about name changes? PutFn(ctx, name, func) ?
	wfn.Fn.Name = fn

	f, err := s.datastore.PutFn(ctx, wfn.Fn)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, fnResponse{"Successfully put fn", f})
}
