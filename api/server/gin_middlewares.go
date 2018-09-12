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
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"
	"strconv"
	"time"
)

var (
	pathKey   = common.MakeKey("path")
	methodKey = common.MakeKey("method")
	statusKey = common.MakeKey("status")

	apiRequestCountMeasure  = common.MakeMeasure("api/request_count", "Count of API requests started", stats.UnitDimensionless)
	apiResponseCountMeasure = common.MakeMeasure("api/response_count", "API response count", stats.UnitDimensionless)
	apiLatencyMeasure       = common.MakeMeasure("api/latency", "Latency distribution of API requests", stats.UnitMilliseconds)
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
		tag.Insert(appKey, c.Param(api.ParamAppName)),
		tag.Insert(pathKey, c.Param(api.ParamRouteName)),
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

func RegisterAPIViews(tagKeys []string, dist []float64) {

	// default tags for request and response
	reqTags := []tag.Key{pathKey, methodKey}
	respTags := []tag.Key{pathKey, methodKey, statusKey}

	// add extra tags if not already in default tags for req/resp
	for _, key := range tagKeys {
		if key != "path" && key != "method" && key != "status" {
			reqTags = append(reqTags, common.MakeKey(key))
			respTags = append(respTags, common.MakeKey(key))
		}
	}

	err := view.Register(
		common.CreateViewWithTags(apiRequestCountMeasure, view.Count(), reqTags),
		common.CreateViewWithTags(apiResponseCountMeasure, view.Count(), respTags),
		common.CreateViewWithTags(apiLatencyMeasure, view.Distribution(dist...), respTags),
	)
	if err != nil {
		logrus.WithError(err).Fatal("cannot register view")
	}
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
			stats.Record(ctx, apiRequestCountMeasure.M(0))
			c.Next()

			status := strconv.Itoa(c.Writer.Status())
			ctx, err = tag.New(ctx,
				tag.Upsert(statusKey, status),
			)
			if err != nil {
				logrus.Fatal(err)
			}
			stats.Record(ctx, apiResponseCountMeasure.M(0))
			stats.Record(ctx, apiLatencyMeasure.M(int64(time.Since(start)/time.Millisecond)))
		}
	}

	r := s.Router
	r.Use(measure(r))
	if s.svcConfigs[WebServer].Addr != s.svcConfigs[AdminServer].Addr {
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

	if appName := c.Param(api.ParamAppName); appName != "" {
		c.Set(api.AppName, appName)
		ctx = ContextWithApp(ctx, appName)
	}

	if routePath := c.Param(api.ParamRouteName); routePath != "" {
		c.Set(api.Path, routePath)
		ctx = ContextWithPath(ctx, routePath)
	}

	c.Request = c.Request.WithContext(ctx)
	c.Next()
}

type ctxPathKey string

// ContextWithPath sets the routePath value on a context, it may be retrieved
// using PathFromContext.
// TODO this is also used as a gin.Key -- stop one of these two things.
func ContextWithPath(ctx context.Context, routePath string) context.Context {
	return context.WithValue(ctx, ctxPathKey(api.Path), routePath)
}

// PathFromContext returns the path from a context, if set.
func PathFromContext(ctx context.Context) string {
	r, _ := ctx.Value(ctxPathKey(api.Path)).(string)
	return r
}

type ctxAppKey string

// ContextWithApp sets the app name value on a context, it may be retrieved
// using AppFromContext.
// TODO this is also used as a gin.Key -- stop one of these two things.
func ContextWithApp(ctx context.Context, app string) context.Context {
	return context.WithValue(ctx, ctxAppKey(api.AppName), app)
}

// AppFromContext returns the app from a context, if set.
func AppFromContext(ctx context.Context) string {
	r, _ := ctx.Value(ctxAppKey(api.AppName)).(string)
	return r
}

func (s *Server) checkAppPresenceByNameAtLB() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, _ := common.LoggerWithFields(c.Request.Context(), extractFields(c))

		appName := c.Param(api.ParamAppName)
		if appName != "" {
			appID, err := s.lbReadAccess.GetAppID(ctx, appName)
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

		appName := c.MustGet(api.AppName).(string)
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
	appName := c.GetString(api.AppName)
	if appName != "" {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), fnext.AppNameKey, appName))
	}
	c.Next()
}

func setAppIDInCtx(c *gin.Context) {
	// add appName to context
	appID := c.Param(api.ParamAppID)

	if appID != "" {
		c.Set(api.AppID, appID)
		c.Request = c.Request.WithContext(c)
	}
	c.Next()
}

func appNameCheck(c *gin.Context) {
	appName := c.GetString(api.AppName)
	if appName == "" {
		handleV1ErrorResponse(c, models.ErrAppsMissingName)
		c.Abort()
		return
	}
	c.Next()
}
