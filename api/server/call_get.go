package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleCallGet(c *gin.Context) {
	ctx := c.Request.Context()

	fnID := c.Param(api.FnID)

	if fnID == "" {
		handleErrorResponse(c, models.ErrFnsMissingID)
		return
	}

	_, err := s.datastore.GetFnByID(ctx, c.Param(api.FnID))
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	callID := c.Param(api.CallID)
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
