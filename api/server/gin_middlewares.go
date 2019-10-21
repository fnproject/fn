// This is middleware we're using for the entire server.

package server

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/common"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"
)

var (
	pathKey         = common.MakeKey("path")
	methodKey       = common.MakeKey("method")
	statusKey       = common.MakeKey("status")
	whodunitKey     = common.MakeKey("blame")
	fnFdkVersionKey = common.MakeKey("fn_fdk_version")

	apiRequestCountMeasure  = common.MakeMeasure("api/request_count", "Count of API requests started", stats.UnitDimensionless)
	apiResponseCountMeasure = common.MakeMeasure("api/response_count", "API response count", stats.UnitDimensionless)
	apiLatencyMeasure       = common.MakeMeasure("api/latency", "Latency distribution of API requests", stats.UnitMilliseconds)

	APIViewsGetPath = DefaultAPIViewsGetPath
)

const (
	fnFdkVersionHeader = "Fn-Fdk-Version"
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
	appIDKey, err := tag.NewKey("fn.app_id")
	if err != nil {
		logrus.Fatal(err)
	}
	fnKey, err := tag.NewKey("fn.fn_id")
	if err != nil {
		logrus.Fatal(err)
	}
	ctx, err := tag.New(c.Request.Context(),
		tag.Insert(appIDKey, c.Param(api.AppID)),
		tag.Insert(fnKey, c.Param(api.FnID)),
	)
	if err != nil {
		logrus.Fatal(err)
	}

	// TODO inspect opencensus more and see if we need to define a header ourselves
	// to trigger per-request spans (we will want this), we can set sampler here per request.

	ctx, span := trace.StartSpan(ctx, "serve_http")
	defer span.End()

	// spans like these, not tags
	span.AddAttributes(
		trace.StringAttribute("fn.app_id", c.Param(api.AppID)),
		trace.StringAttribute("fn.fn_id", c.Param(api.FnID)),
	)

	c.Request = c.Request.WithContext(ctx)
	c.Next()
}

func RegisterAPIViews(tagKeys []string, dist []float64) {

	// default tags for request and response
	reqTags := []tag.Key{pathKey, methodKey}
	respTags := []tag.Key{pathKey, methodKey, statusKey, whodunitKey, fnFdkVersionKey}

	// add extra tags if not already in default tags for req/resp
	for _, key := range tagKeys {
		if key != pathKey.Name() && key != methodKey.Name() && key != statusKey.Name() && key != whodunitKey.Name() && key != fnFdkVersionKey.Name() {
			respTags = append(respTags, common.MakeKey(key))
		}
		if key != pathKey.Name() && key != methodKey.Name() {
			reqTags = append(reqTags, common.MakeKey(key))
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

func DefaultAPIViewsGetPath(routes gin.RoutesInfo, c *gin.Context) string {
	// get the handler url, example: /v2/apps/:app
	url := "invalid"
	for _, r := range routes {
		if r.Handler == c.HandlerName() {
			url = r.Path
			break
		}
	}
	return url
}

func apiMetricsWrap(s *Server) {
	measure := func(engine *gin.Engine) func(*gin.Context) {
		var routes gin.RoutesInfo
		return func(c *gin.Context) {
			if routes == nil {
				routes = engine.Routes()
			}
			start := time.Now()
			ctx, err := tag.New(c.Request.Context(),
				tag.Upsert(pathKey, APIViewsGetPath(routes, c)),
				tag.Upsert(methodKey, c.Request.Method),
			)
			if err != nil {
				logrus.Fatal(err)
			}
			stats.Record(ctx, apiRequestCountMeasure.M(0))
			c.Next()

			status := strconv.Itoa(c.Writer.Status())

			ctx, err = tag.New(c.Request.Context(), // important, request context could be mutated by now
				tag.Upsert(pathKey, APIViewsGetPath(routes, c)),
				tag.Upsert(methodKey, c.Request.Method),
				tag.Upsert(statusKey, status),
				tag.Insert(whodunitKey, "service"), // only insert this if it doesn't exist
				tag.Upsert(fnFdkVersionKey, c.Writer.Header().Get(fnFdkVersionHeader)),
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
			handleErrorResponse(c, err)
			c.Abort()
		}
	}(c)
	c.Next()
}

func loggerWrap(c *gin.Context) {
	ctx, _ := common.LoggerWithFields(c.Request.Context(), extractFields(c))

	if appID := c.Param(api.AppID); appID != "" {
		c.Set(api.AppID, appID)
		ctx = ContextWithAppID(ctx, appID)
	}

	if fnID := c.Param(api.FnID); fnID != "" {
		c.Set(api.FnID, fnID)
		ctx = ContextWithFnID(ctx, fnID)
	}

	c.Request = c.Request.WithContext(ctx)
	c.Next()
}

type ctxFnIDKey string

func ContextWithFnID(ctx context.Context, fnID string) context.Context {
	return context.WithValue(ctx, ctxFnIDKey(api.FnID), fnID)
}

// FnIDFromContext returns the app from a context, if set.
func FnIDFromContext(ctx context.Context) string {
	r, _ := ctx.Value(ctxFnIDKey(api.FnID)).(string)
	return r
}

type ctxAppIDKey string

func ContextWithAppID(ctx context.Context, appID string) context.Context {
	return context.WithValue(ctx, ctxAppIDKey(api.AppID), appID)
}

// AppIDFromContext returns the app from a context, if set.
func AppIDFromContext(ctx context.Context) string {
	r, _ := ctx.Value(ctxAppIDKey(api.AppID)).(string)
	return r
}
