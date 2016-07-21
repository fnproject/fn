package router

import (
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
)

func handleAppCreate(c *gin.Context) {
	store := c.MustGet("store").(models.Datastore)
	log := c.MustGet("log").(logrus.FieldLogger)

	app := &models.App{}

	err := c.BindJSON(app)
	if err != nil {
		log.WithError(err).Debug(models.ErrInvalidJSON)
		c.JSON(http.StatusBadRequest, simpleError(models.ErrInvalidJSON))
		return
	}

	app, err = store.StoreApp(app)
	if err != nil {
		log.WithError(err).Debug(models.ErrAppsCreate)
		c.JSON(http.StatusInternalServerError, simpleError(models.ErrAppsCreate))
		return
	}

	c.JSON(http.StatusOK, app)
}
