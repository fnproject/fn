package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleCallGet(c *gin.Context) {
	ctx := c.Request.Context()

	callID := c.Param(api.Call)
	app, err := s.datastore.GetApp(ctx, &models.App{Name: c.MustGet(api.App).(string)})
	if err != nil {
		handleErrorResponse(c, err)
		return
	}
	callObj, err := s.datastore.GetCall(ctx, app.ID, callID)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, callResponse{"Successfully loaded call", callObj})
}
