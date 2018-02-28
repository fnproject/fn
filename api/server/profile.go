package server

import (
	"expvar"
	"net/http/pprof"

	"github.com/gin-gonic/gin"
)

func profilerSetup(router *gin.Engine, path string) {
	engine := router.Group(path)
	engine.Any("/vars", gin.WrapF(expvar.Handler().ServeHTTP))
	engine.Any("/pprof/", gin.WrapF(pprof.Index))
	engine.Any("/pprof/cmdline", gin.WrapF(pprof.Cmdline))
	engine.Any("/pprof/profile", gin.WrapF(pprof.Profile))
	engine.Any("/pprof/symbol", gin.WrapF(pprof.Symbol))
	engine.Any("/pprof/block", gin.WrapF(pprof.Handler("block").ServeHTTP))
	engine.Any("/pprof/heap", gin.WrapF(pprof.Handler("heap").ServeHTTP))
	engine.Any("/pprof/goroutine", gin.WrapF(pprof.Handler("goroutine").ServeHTTP))
	engine.Any("/pprof/threadcreate", gin.WrapF(pprof.Handler("threadcreate").ServeHTTP))
}
