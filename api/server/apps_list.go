package server

import (
	"fmt"
	"net/http"

	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleAppList(c *gin.Context) {
	ctx := c.Request.Context()

	filter := &models.AppFilter{}

	filter.Cursor, filter.PerPage = pageParams(c)

	filter.Name = c.Query("name")

	apps, err := s.datastore.GetApps(ctx, filter)
	for _, item := range apps.Items {
		fmt.Println("~~app get app : " + item.ID)
		fmt.Println("~~app get app : " + item.Name)
		fmt.Println(item.Architecture)
	}
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, apps)
}
