package router

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
)

func handleRouteCreate(c *gin.Context) {
	store := c.MustGet("store").(models.Datastore)
	log := c.MustGet("log").(logrus.FieldLogger)

	route := &models.Route{}

	err := c.BindJSON(route)
	if err != nil {
		log.WithError(err).Debug(models.ErrInvalidJSON)
		c.JSON(http.StatusBadRequest, simpleError(models.ErrInvalidJSON))
		return
	}

	route.AppName = c.Param("app")

	route, err = store.StoreRoute(route)
	if err != nil {
		log.WithError(err).Debug(models.ErrRoutesCreate)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrRoutesCreate))
		return
	}

	c.JSON(http.StatusOK, route)
}
