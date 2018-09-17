package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleCallGet(c *gin.Context) {
	ctx := c.Request.Context()

	fnID := c.Param(api.ParamFnID)

	if fnID == "" {
		handleErrorResponse(c, models.ErrFnsMissingID)
		return
	}

	_, err := s.datastore.GetFnByID(ctx, c.Param(api.ParamFnID))
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	callID := c.Param(api.ParamCallID)
	if callID == "" {
		handleErrorResponse(c, models.ErrDatastoreEmptyCallID)
	}

	callObj, err := s.logstore.GetCall(ctx, fnID, callID)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, callObj)
}
