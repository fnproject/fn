package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/fnproject/fn/api/models"
	"github.com/gin-gonic/gin"
)

func (s *Server) bindHandlers(ctx context.Context) {
	engine := s.Router
	// now for extensible middleware
	engine.Use(s.rootMiddlewareWrapper())

	engine.GET("/", handlePing)
	engine.GET("/version", handleVersion)

	// TODO: move under v1 ?
	if s.promExporter != nil {
		engine.GET("/metrics", gin.WrapH(s.promExporter))
	}

	profilerSetup(engine, "/debug")

	// Pure runners don't have any route, they have grpc
	if s.nodeType != ServerTypePureRunner {
		if s.nodeType != ServerTypeRunner {
			clean := engine.Group("/v1")
			v1 := clean.Group("")
			v1.Use(setAppNameInCtx)
			v1.Use(s.apiMiddlewareWrapper())
			v1.GET("/apps", s.handleAppList)
			v1.POST("/apps", s.handleAppCreate)

			{
				apps := v1.Group("/apps/:app")
				apps.Use(appNameCheck)

				{
					withAppCheck := apps.Group("")
					withAppCheck.Use(s.checkAppPresenceByName())
					withAppCheck.GET("", s.handleAppGetByName)
					withAppCheck.PATCH("", s.handleAppUpdate)
					withAppCheck.DELETE("", s.handleAppDelete)
					withAppCheck.GET("/routes", s.handleRouteList)
					withAppCheck.GET("/routes/:route", s.handleRouteGetAPI)
					withAppCheck.PATCH("/routes/*route", s.handleRoutesPatch)
					withAppCheck.DELETE("/routes/*route", s.handleRouteDelete)
					withAppCheck.GET("/calls/:call", s.handleCallGet)
					withAppCheck.GET("/calls/:call/log", s.handleCallLogGet)
					withAppCheck.GET("/calls", s.handleCallList)
				}

				apps.POST("/routes", s.handleRoutesPostPut)
				apps.PUT("/routes/*route", s.handleRoutesPostPut)
			}

			{
				runner := clean.Group("/runner")
				runner.PUT("/async", s.handleRunnerEnqueue)
				runner.GET("/async", s.handleRunnerDequeue)

				runner.POST("/start", s.handleRunnerStart)
				runner.POST("/finish", s.handleRunnerFinish)

				appsAPIV2 := runner.Group("/apps/:app")
				appsAPIV2.Use(setAppNameInCtx)
				appsAPIV2.GET("", s.handleAppGetByID)
				appsAPIV2.GET("/routes/:route", s.handleRouteGetRunner)

			}
		}

		if s.nodeType != ServerTypeAPI {
			runner := engine.Group("/r")
			runner.Use(s.checkAppPresenceByNameAtRunner())
			runner.Any("/:app", s.handleFunctionCall)
			runner.Any("/:app/*route", s.handleFunctionCall)
		}

	}

	engine.NoRoute(func(c *gin.Context) {
		var err error
		switch {
		case s.nodeType == ServerTypeAPI && strings.HasPrefix(c.Request.URL.Path, "/r/"):
			err = models.ErrInvokeNotSupported
		case s.nodeType == ServerTypeRunner && strings.HasPrefix(c.Request.URL.Path, "/v1/"):
			err = models.ErrAPINotSupported
		default:
			var e models.APIError = models.ErrPathNotFound
			err = models.NewAPIError(e.Code(), fmt.Errorf("%v: %s", e.Error(), c.Request.URL.Path))
		}
		handleErrorResponse(c, err)
	})

}
