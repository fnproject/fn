package server

import (
	"net/http"

	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleFuncsList(c *gin.Context) {
	ctx := c.Request.Context()

	var filter models.FuncFilter
	filter.Cursor, filter.PerPage = pageParams(c, false)

	funcs, err := s.datastore.GetFuncs(ctx, &filter)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	var nextCursor string
	if len(funcs) > 0 && len(funcs) == filter.PerPage {
		nextCursor = funcs[len(funcs)-1].ID
	}

	c.JSON(http.StatusOK, funcsResponse{
		Message:    "Successfully listed applications",
		NextCursor: nextCursor,
		Funcs:      funcs,
	})
}
