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

	appName := c.Param(api.CApp)
	fnName := c.Param(api.Fn)
	wfn.Fn.Name = fnName

	appID, err := s.datastore.GetAppID(ctx, appName)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}
	wfn.Fn.AppID = appID

	_, err = s.datastore.GetFn(ctx, appID, fnName)

	if err != nil && err != models.ErrFnsNotFound {
		handleErrorResponse(c, err)
	}

	var f *models.Fn
	if err == models.ErrFnsNotFound {
		f, err = s.datastore.InsertFn(ctx, wfn.Fn)
	} else {
		f, err = s.datastore.UpdateFn(ctx, wfn.Fn)
	}
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, fnResponse{"Successfully put fn", f})
}