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
	name := c.Query("name")
	if name != "" {
		filter.NameIn = []string{name}
	}

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

	c.JSON(http.StatusOK, appListResponse{
		NextCursor: nextCursor,
		Items:      apps,
	})
}
