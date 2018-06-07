package server

import (
	"net/http"

	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleFnsList(c *gin.Context) {
	ctx := c.Request.Context()

	var filter models.FnFilter
	filter.Cursor, filter.PerPage = pageParams(c, false)

	fns, err := s.datastore.GetFns(ctx, &filter)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	var nextCursor string
	if len(fns) > 0 && len(fns) == filter.PerPage {
		nextCursor = fns[len(fns)-1].ID
	}

	c.JSON(http.StatusOK, fnsResponse{
		Message:    "Successfully listed applications",
		NextCursor: nextCursor,
		Fns:        fns,
	})
}
