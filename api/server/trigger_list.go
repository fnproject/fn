package server

import (
	"net/http"

	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleTriggerList(c *gin.Context) {
	ctx := c.Request.Context()

	filter := &models.TriggerFilter{}
	filter.Cursor, filter.PerPage = pageParamsV2(c)

	filter.AppID = c.Query("app_id")

	if filter.AppID == "" {
		handleErrorResponse(c, models.ErrTriggerMissingAppID)
	}

	filter.FnID = c.Query("fn_id")
	filter.Name = c.Query("name")

	triggers, err := s.datastore.GetTriggers(ctx, filter)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, triggers)
}
