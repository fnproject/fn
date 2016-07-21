package router

import (
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
)

func Start(engine *gin.Engine) {
	engine.GET("/", handlePing)
	engine.GET("/version", handleVersion)

	v1 := engine.Group("/v1")
	{
		v1.GET("/apps", handleAppList)
		v1.POST("/apps", handleAppCreate)

		v1.GET("/apps/:app", handleAppGet)
		v1.POST("/apps/:app", handleAppUpdate)
		v1.DELETE("/apps/:app", handleAppDestroy)

		apps := v1.Group("/apps/:app")
		{
			apps.GET("/routes", handleRouteList)
			apps.POST("/routes", handleRouteCreate)
			apps.GET("/routes/:route", handleRouteGet)
			apps.POST("/routes/:route", handleRouteUpdate)
			apps.DELETE("/routes/:route", handleRouteDestroy)
		}

	}

	engine.GET("/r/:app/*route", handleRunner)
}

func simpleError(err error) *models.Error {
	return &models.Error{&models.ErrorBody{Message: err.Error()}}
}
