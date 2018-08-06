package server

import (
	"fmt"
	"net/http"

	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

//TODO deprecate with V2
func (s *Server) handleV1AppCreate(c *gin.Context) {
	ctx := c.Request.Context()

	var wapp models.AppWrapper

	err := c.BindJSON(&wapp)
	if err != nil {
		fmt.Println("YODAWG", err)
		if models.IsAPIError(err) {
			handleV1ErrorResponse(c, err)
		} else {
			handleV1ErrorResponse(c, models.ErrInvalidJSON)
		}
		return
	}

	app := wapp.App
	if app == nil {
		handleV1ErrorResponse(c, models.ErrAppsMissingNew)
		return
	}

	app, err = s.datastore.InsertApp(ctx, app)
	if err != nil {
		handleV1ErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, appResponse{"App successfully created", app})
}
