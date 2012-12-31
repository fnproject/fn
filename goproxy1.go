package main

import (
	"flag"
	"github.com/elazarl/goproxy"
	"log"
	"net/http"
)

func main() {
	verbose := flag.Bool("v", true, "should every proxy request be logged to stdout")
	flag.Parse()
	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = *verbose
	proxy.OnRequest().DoFunc(
		func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			r.Header.Set("X-GoProxy", "yxorPoG-X")
			return r, nil
		})

	log.Fatal(http.ListenAndServe(":8080", proxy))
}
