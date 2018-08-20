package server

import (
	"net/http"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleFnCreate(c *gin.Context) {
	ctx := c.Request.Context()
	log := common.Logger(ctx)

	fn := &models.Fn{}
	err := c.BindJSON(fn)
	if err != nil {
		if !models.IsAPIError(err) {
			err = models.ErrInvalidJSON
		}
		handleErrorResponse(c, err)
		return
	}

	fn.SetDefaults()
	fnCreated, err := s.datastore.InsertFn(ctx, fn)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	app, err := s.datastore.GetAppByID(ctx, fnCreated.AppID)
	if err != nil {
		log.Debugln("Failed to lookup app.")
		c.JSON(http.StatusOK, fnCreated)
		return
	}

	fnAnnotated, err := s.fnAnnotator.AnnotateFn(c, app, fnCreated)
	if err != nil {
		log.Debugln("Failed to annotate fn")
		c.JSON(http.StatusOK, fnCreated)
		return
	}

	c.JSON(http.StatusOK, fnAnnotated)
}
