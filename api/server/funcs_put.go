package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleFuncsPut(c *gin.Context) {
	ctx := c.Request.Context()

	var wfunc models.FuncWrapper
	err := c.BindJSON(&wfunc)
	if err != nil {
		if !models.IsAPIError(err) {
			// TODO this error message sucks
			err = models.ErrInvalidJSON
		}
		handleErrorResponse(c, err)
		return
	}
	if wfunc.Func == nil {
		handleErrorResponse(c, models.ErrFuncsMissingNew)
		return
	}

	fn := c.Param(api.Func)
	// TODO: what about name changes? PutFunc(ctx, name, func) ?
	wfunc.Func.Name = fn

	f, err := s.datastore.PutFunc(ctx, wfunc.Func)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, funcResponse{"Successfully put func", f})
}
