package server

import (
	"encoding/base64"
	"net/http"

	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleAppList(c *gin.Context) {
	ctx := c.Request.Context()

	filter := &models.AppFilter{}
	filter.Cursor, filter.PerPage = pageParams(c, true)

	apps, err := s.datastore.GetApps(ctx, filter)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	var nextCursor string
	if len(apps) > 0 && len(apps) == filter.PerPage {
		last := []byte(apps[len(apps)-1].Name)
		nextCursor = base64.RawURLEncoding.EncodeToString(last)
	}

	c.JSON(http.StatusOK, appsResponse{
		Message:    "Successfully listed applications",
		NextCursor: nextCursor,
		Apps:       apps,
	})
}
