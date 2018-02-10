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
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/sirupsen/logrus"
)

func optionalCorsWrap(r *gin.Engine) {
	// By default no CORS are allowed unless one
	// or more Origins are defined by the API_CORS
	// environment variable.
	corsStr := getEnv(EnvAPICORS, "")
	if len(corsStr) > 0 {
		origins := strings.Split(strings.Replace(corsStr, " ", "", -1), ",")

		corsConfig := cors.DefaultConfig()
		corsConfig.AllowOrigins = origins

		logrus.Infof("CORS enabled for domains: %s", origins)

		r.Use(cors.New(corsConfig))
	}
}

// we should use http grr
func traceWrap(c *gin.Context) {
	// try to grab a span from the request if made from another service, ignore err if not
	wireContext, _ := opentracing.GlobalTracer().Extract(
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(c.Request.Header))

	// Create the span referring to the RPC client if available.
	// If wireContext == nil, a root span will be created.
	// TODO we should add more tags?
	serverSpan := opentracing.StartSpan("serve_http", ext.RPCServerOption(wireContext), opentracing.Tag{Key: "path", Value: c.Request.URL.Path})
	serverSpan.SetBaggageItem("fn_appname", c.Param(api.CApp))
	serverSpan.SetBaggageItem("fn_path", c.Param(api.CRoute))
	defer serverSpan.Finish()

	ctx := tracing.WithSpan(c.Request.Context(), serverSpan)
	c.Request = c.Request.WithContext(ctx)
	c.Next()
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

	if appName := c.Param(api.CApp); appName != "" {
		c.Set(api.AppName, appName)
		ctx = context.WithValue(ctx, api.AppName, appName)
	}

	if routePath := c.Param(api.CRoute); routePath != "" {
		c.Set(api.Path, routePath)
		ctx = context.WithValue(ctx, api.Path, routePath)
	}

	c.Request = c.Request.WithContext(ctx)
	c.Next()
}

func setAppNameInCtx(c *gin.Context) {
	// add appName to context
	appName := c.GetString(api.AppName)
	if appName != "" {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), fnext.AppNameKey, appName))
	}
	c.Next()
}

func appNameCheck(c *gin.Context) {
	appName := c.GetString(api.AppName)
	if appName == "" {
		handleErrorResponse(c, models.ErrAppsMissingName)
		c.Abort()
		return
	}
	c.Next()
}
