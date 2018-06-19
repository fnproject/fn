package server

import (
	"encoding/base64"
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleTriggerList(c *gin.Context) {
	ctx := c.Request.Context()

	filter := &models.TriggerFilter{}
	filter.Cursor, filter.PerPage = pageParams(c, true)

	filter.AppID = c.MustGet(api.AppID).(string)

	//TODO when FN's merged
	//	filter.fnName = c.Query("fnName")

	triggers, err := s.datastore.GetTriggers(ctx, filter)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	var nextCursor string
	if len(triggers) > 0 && len(triggers) == filter.PerPage {
		last := []byte(triggers[len(triggers)-1].Name)
		nextCursor = base64.RawURLEncoding.EncodeToString(last)
	}

	c.JSON(http.StatusOK, triggersResponse{
		Message:    "Successfully listed triggersp",
		NextCursor: nextCursor,
		Triggers:   triggers,
	})
}
