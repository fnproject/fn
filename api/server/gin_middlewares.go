// This is middleware we're using for the entire server.

package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/fnext"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"
	"strconv"
	"time"
)

func optionalCorsWrap(r *gin.Engine) {
	// By default no CORS are allowed unless one
	// or more Origins are defined by the API_CORS
	// environment variable.
	corsStr := getEnv(EnvAPICORSOrigins, "")
	if len(corsStr) > 0 {
		origins := strings.Split(strings.Replace(corsStr, " ", "", -1), ",")

		corsConfig := cors.DefaultConfig()
		if origins[0] == "*" {
			corsConfig.AllowAllOrigins = true
		} else {
			corsConfig.AllowOrigins = origins
		}

		corsHeaders := getEnv(EnvAPICORSHeaders, "")
		if len(corsHeaders) > 0 {
			headers := strings.Split(strings.Replace(corsHeaders, " ", "", -1), ",")
			corsConfig.AllowHeaders = headers
		}

		corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "HEAD", "DELETE"}

		logrus.Infof("CORS enabled for domains: %s", origins)

		r.Use(cors.New(corsConfig))
	}
}

// we should use http grr
func traceWrap(c *gin.Context) {
	appKey, err := tag.NewKey("fn_appname")
	if err != nil {
		logrus.Fatal(err)
	}
	pathKey, err := tag.NewKey("fn_path")
	if err != nil {
		logrus.Fatal(err)
	}
	ctx, err := tag.New(c.Request.Context(),
		tag.Insert(appKey, c.Param(api.CApp)),
		tag.Insert(pathKey, c.Param(api.CRoute)),
	)
	if err != nil {
		logrus.Fatal(err)
	}

	// TODO inspect opencensus more and see if we need to define a header ourselves
	// to trigger per-request spans (we will want this), we can set sampler here per request.

	ctx, serverSpan := trace.StartSpan(ctx, "serve_http")
	defer serverSpan.End()

	c.Request = c.Request.WithContext(ctx)
	c.Next()
}

func apiMetricsWrap(s *Server) {

	measure := func(engine *gin.Engine) func(*gin.Context) {
		var routes gin.RoutesInfo
		return func(c *gin.Context) {
			if routes == nil {
				routes = engine.Routes()
			}
			start := time.Now()
			// get the handler url, example: /v1/apps/:app
			url := "invalid"
			for _, r := range routes {
				if r.Handler == c.HandlerName() {
					url = r.Path
					break
				}
			}

			ctx, err := tag.New(c.Request.Context(),
				tag.Upsert(pathKey, url),
				tag.Upsert(methodKey, c.Request.Method),
			)
			if err != nil {
				logrus.Fatal(err)
			}
			stats.Record(ctx, apiRequestCount.M(1))
			c.Next()

			status := strconv.Itoa(c.Writer.Status())
			ctx, err = tag.New(ctx,
				tag.Upsert(statusKey, status),
			)
			if err != nil {
				logrus.Fatal(err)
			}
			stats.Record(ctx, apiLatency.M(float64(time.Since(start))/float64(time.Millisecond)))
		}
	}

	r := s.Router
	r.Use(measure(r))
	if s.webListenPort != s.adminListenPort {
		a := s.AdminRouter
		a.Use(measure(a))
	}

}

func panicWrap(c *gin.Context) {
	defer func(c *gin.Context) {
		if rec := recover(); rec != nil {
			err, ok := rec.(error)
			if !ok {
				err = fmt.Errorf("fn: %v", rec)
			}
			handleV1ErrorResponse(c, err)
			c.Abort()
		}
	}(c)
	c.Next()
}

func loggerWrap(c *gin.Context) {
	ctx, _ := common.LoggerWithFields(c.Request.Context(), extractFields(c))

	if appName := c.Param(api.CApp); appName != "" {
		c.Set(api.App, appName)
		ctx = context.WithValue(ctx, api.App, appName)
	}

	if routePath := c.Param(api.CRoute); routePath != "" {
		c.Set(api.Path, routePath)
		ctx = context.WithValue(ctx, api.Path, routePath)
	}

	c.Request = c.Request.WithContext(ctx)
	c.Next()
}

func (s *Server) checkAppPresenceByNameAtRunner() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, _ := common.LoggerWithFields(c.Request.Context(), extractFields(c))

		appName := c.Param(api.CApp)
		if appName != "" {
			appID, err := s.agent.GetAppID(ctx, appName)
			if err != nil {
				handleV1ErrorResponse(c, err)
				c.Abort()
				return
			}
			c.Set(api.AppID, appID)
		}

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func (s *Server) checkAppPresenceByName() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, _ := common.LoggerWithFields(c.Request.Context(), extractFields(c))

		appName := c.MustGet(api.App).(string)
		if appName != "" {
			appID, err := s.datastore.GetAppID(ctx, appName)
			if err != nil {
				handleV1ErrorResponse(c, err)
				c.Abort()
				return
			}
			c.Set(api.AppID, appID)
		}

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func setAppNameInCtx(c *gin.Context) {
	// add appName to context
	appName := c.GetString(api.App)
	if appName != "" {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), fnext.AppNameKey, appName))
	}
	c.Next()
}

func appNameCheck(c *gin.Context) {
	appName := c.GetString(api.App)
	if appName == "" {
		handleV1ErrorResponse(c, models.ErrAppsMissingName)
		c.Abort()
		return
	}
	c.Next()
}
