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
	filter.Name = c.Query("name")

	apps, err := s.datastore.GetApps(ctx, filter)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	if len(apps.Items) > 0 && len(apps.Items) == filter.PerPage {
		last := []byte(apps.Items[len(apps.Items)-1].Name)
		apps.NextCursor = base64.RawURLEncoding.EncodeToString(last)
	}

	c.JSON(http.StatusOK, apps)
}
