package server

import (
	"expvar"
	"fmt"
	"net/http"
	"net/http/pprof"

	"github.com/gin-gonic/gin"
)

// Replicated from expvar.go as not public.
func expVars(w http.ResponseWriter, r *http.Request) {
	first := true
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintf(w, "{\n")
	expvar.Do(func(kv expvar.KeyValue) {
		if !first {
			fmt.Fprintf(w, ",\n")
		}
		first = false
		fmt.Fprintf(w, "%q: %s", kv.Key, kv.Value)
	})
	fmt.Fprintf(w, "\n}\n")
}

func profilerSetup(router *gin.Engine, path string) {
	engine := router.Group(path)
	engine.Any("/vars", gin.WrapF(expVars))
	engine.Any("/pprof/", gin.WrapF(pprof.Index))
	engine.Any("/pprof/cmdline", gin.WrapF(pprof.Cmdline))
	engine.Any("/pprof/profile", gin.WrapF(pprof.Profile))
	engine.Any("/pprof/symbol", gin.WrapF(pprof.Symbol))
	engine.Any("/pprof/block", gin.WrapF(pprof.Handler("block").ServeHTTP))
	engine.Any("/pprof/heap", gin.WrapF(pprof.Handler("heap").ServeHTTP))
	engine.Any("/pprof/goroutine", gin.WrapF(pprof.Handler("goroutine").ServeHTTP))
	engine.Any("/pprof/threadcreate", gin.WrapF(pprof.Handler("threadcreate").ServeHTTP))
}
