package server

import (
	"net/http"
	"strings"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleFnsPut(c *gin.Context) {
	ctx := c.Request.Context()
	method := strings.ToUpper(c.Request.Method)

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

	appName := c.MustGet(api.App).(string)
	appID, err := s.ensureApp(ctx, appName, method)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	fn := c.Param(api.Fn)
	// TODO: what about name changes? PutFn(ctx, name, func) ?
	wfn.Fn.AppID = appID
	wfn.Fn.Name = fn

	f, err := s.datastore.PutFn(ctx, wfn.Fn)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, fnResponse{"Successfully put fn", f})
}
