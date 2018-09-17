package server

import (
	"net/http"

	"fmt"

	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleTriggerList(c *gin.Context) {
	ctx := c.Request.Context()

	filter := &models.TriggerFilter{}
	filter.Cursor, filter.PerPage = pageParams(c)

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

	// Annotate the outbound triggers
	// this is fairly cludgy bit hard to do in datastore middleware confidently
	appCache := make(map[string]*models.App)

	for idx, t := range triggers.Items {
		app, ok := appCache[t.AppID]
		if !ok {
			gotApp, err := s.Datastore().GetAppByID(ctx, t.AppID)
			if err != nil {
				handleErrorResponse(c, fmt.Errorf("failed to get app for trigger %s", err))
				return
			}
			app = gotApp
			appCache[app.ID] = gotApp
		}

		newT, err := s.triggerAnnotator.AnnotateTrigger(c, app, t)
		if err != nil {
			handleErrorResponse(c, err)
			return
		}
		triggers.Items[idx] = newT
	}

	c.JSON(http.StatusOK, triggers)
}
