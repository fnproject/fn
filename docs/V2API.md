goroutine 153 [running]:
runtime/debug.Stack(0xc4200b21e0, 0x4e03a20, 0xc42045b040)
    /usr/local/go/src/runtime/debug/stack.go:24 +0xa7
github.com/fnproject/fn/api/server.handleV1ErrorResponse(0xc42061c000, 0x4e03a20, 0xc42045b040)
    /Users/OCliffe/gows/src/github.com/fnproject/fn/api/server/error_response.go:44 +0x498
github.com/fnproject/fn/api/server.panicWrap.func1(0xc42061c000)
    /Users/OCliffe/gows/src/github.com/fnproject/fn/api/server/gin_middlewares.go:136 +0x8d
panic(0x4b14b80, 0xc42045b030)
    /usr/local/go/src/runtime/panic.go:505 +0x229
github.com/fnproject/fn/vendor/github.com/gin-gonic/gin.(*Context).MustGet(0xc42061c000, 0x4cfe7d5, 0x3, 0x561cd90, 0x0)
    /Users/OCliffe/gows/src/github.com/fnproject/fn/vendor/github.com/gin-gonic/gin/context.go:192 +0x113
github.com/fnproject/fn/api/server.(*Server).handleV1AppGetByName(0xc42032ab40, 0xc42061c000)
    /Users/OCliffe/gows/src/github.com/fnproject/fn/api/server/apps_v1_get.go:14 +0x6c
github.com/fnproject/fn/api/server.(*Server).(github.com/fnproject/fn/api/server.handleV1AppGetByName)-fm(0xc42061c000)
    /Users/OCliffe/gows/src/github.com/fnproject/fn/api/server/server.go:902 +0x34
github.com/fnproject/fn/vendor/github.com/gin-gonic/gin.(*Context).Next(0xc42061c000)
    /Users/OCliffe/gows/src/github.com/fnproject/fn/vendor/github.com/gin-gonic/gin/context.go:104 +0x43
github.com/fnproject/fn/api/server.setAppNameInCtx(0xc42061c000)
    /Users/OCliffe/gows/src/github.com/fnproject/fn/api/server/gin_middlewares.go:206 +0x1b3
github.com/fnproject/fn/vendor/github.com/gin-gonic/gin.(*Context).Next(0xc42061c000)
    /Users/OCliffe/gows/src/github.com/fnproject/fn/vendor/github.com/gin-gonic/gin/context.go:104 +0x43
github.com/fnproject/fn/api/server.(*Server).runMiddleware(0xc42032ab40, 0xc42061c000, 0x0, 0x0, 0x0)
    /Users/OCliffe/gows/src/github.com/fnproject/fn/api/server/middleware.go:73 +0x29f
github.com/fnproject/fn/api/server.(*Server).rootMiddlewareWrapper.func1(0xc42061c000)
    /Users/OCliffe/gows/src/github.com/fnproject/fn/api/server/middleware.go:63 +0x58
github.com/fnproject/fn/vendor/github.com/gin-gonic/gin.(*Context).Next(0xc42061c000)
    /Users/OCliffe/gows/src/github.com/fnproject/fn/vendor/github.com/gin-gonic/gin/context.go:104 +0x43
github.com/fnproject/fn/api/server.apiMetricsWrap.func1.1(0xc42061c000)
    /Users/OCliffe/gows/src/github.com/fnproject/fn/api/server/gin_middlewares.go:107 +0x3d4
github.com/fnproject/fn/vendor/github.com/gin-gonic/gin.(*Context).Next(0xc42061c000)
    /Users/OCliffe/gows/src/github.com/fnproject/fn/vendor/github.com/gin-gonic/gin/context.go:104 +0x43
github.com/fnproject/fn/api/server.panicWrap(0xc42061c000)
    /Users/OCliffe/gows/src/github.com/fnproject/fn/api/server/gin_middlewares.go:140 +0x51
github.com/fnproject/fn/vendor/github.com/gin-gonic/gin.(*Context).Next(0xc42061c000)
    /Users/OCliffe/gows/src/github.com/fnproject/fn/vendor/github.com/gin-gonic/gin/context.go:104 +0x43
github.com/fnproject/fn/api/server.traceWrap(0xc42061c000)
    /Users/OCliffe/gows/src/github.com/fnproject/fn/api/server/gin_middlewares.go:78 +0x3da
github.com/fnproject/fn/vendor/github.com/gin-gonic/gin.(*Context).Next(0xc42061c000)
    /Users/OCliffe/gows/src/github.com/fnproject/fn/vendor/github.com/gin-gonic/gin/context.go:104 +0x43
github.com/fnproject/fn/api/server.loggerWrap(0xc42061c000)
    /Users/OCliffe/gows/src/github.com/fnproject/fn/api/server/gin_middlewares.go:157 +0x18e
github.com/fnproject/fn/vendor/github.com/gin-gonic/gin.(*Context).Next(0xc42061c000)
    /Users/OCliffe/gows/src/github.com/fnproject/fn/vendor/github.com/gin-gonic/gin/context.go:104 +0x43
github.com/fnproject/fn/vendor/github.com/gin-gonic/gin.(*Engine).handleHTTPRequest(0xc42032aa20, 0xc42061c000)
    /Users/OCliffe/gows/src/github.com/fnproject/fn/vendor/github.com/gin-gonic/gin/gin.go:332 +0x585
github.com/fnproject/fn/vendor/github.com/gin-gonic/gin.(*Engine).ServeHTTP(0xc42032aa20, 0x4e0da40, 0xc420138c40, 0xc420619900)
    /Users/OCliffe/gows/src/github.com/fnproject/fn/vendor/github.com/gin-gonic/gin/gin.go:296 +0x153
github.com/fnproject/fn/vendor/go.opencensus.io/plugin/ochttp.(*Handler).ServeHTTP(0xc420464480, 0x4e0da40, 0xc420138c40, 0xc420619900)
    /Users/OCliffe/gows/src/github.com/fnproject/fn/vendor/go.opencensus.io/plugin/ochttp/server.go:75 +0x109
net/http.serverHandler.ServeHTTP(0xc42031a000, 0x4e0dfc0, 0xc420338700, 0xc420619800)
    /usr/local/go/src/net/http/server.go:2694 +0xbc
net/http.(*conn).serve(0xc4204a63c0, 0x4e0efc0, 0xc420464b00)
    /usr/local/go/src/net/http/server.go:1830 +0x651
created by net/http.(*Server).Serve
    /usr/local/go/src/net/http/server.go:2795 +0x27b

goroutine 250 [running]:
runtime/debug.Stack(0xc4200b81e0, 0x4e03c20, 0xc420117420)
   /usr/local/go/src/runtime/debug/stack.go:24 +0xa7
github.com/fnproject/fn/api/server.handleV1ErrorResponse(0xc4200c2790, 0x4e03c20, 0xc420117420)
   /Users/OCliffe/gows/src/github.com/fnproject/fn/api/server/error_response.go:44 +0x498
github.com/fnproject/fn/api/server.(*Server).checkAppPresenceByNameAtRunner.func1(0xc4200c2790)
   /Users/OCliffe/gows/src/github.com/fnproject/fn/api/server/gin_middlewares.go:168 +0x2bf
github.com/fnproject/fn/vendor/github.com/gin-gonic/gin.(*Context).Next(0xc4200c2790)
   /Users/OCliffe/gows/src/github.com/fnproject/fn/vendor/github.com/gin-gonic/gin/context.go:104 +0x43
github.com/fnproject/fn/api/server.(*Server).runMiddleware(0xc4204465a0, 0xc4200c2790, 0x0, 0x0, 0x0)
   /Users/OCliffe/gows/src/github.com/fnproject/fn/api/server/middleware.go:73 +0x29f
github.com/fnproject/fn/api/server.(*Server).rootMiddlewareWrapper.func1(0xc4200c2790)
   /Users/OCliffe/gows/src/github.com/fnproject/fn/api/server/middleware.go:63 +0x58
github.com/fnproject/fn/vendor/github.com/gin-gonic/gin.(*Context).Next(0xc4200c2790)
   /Users/OCliffe/gows/src/github.com/fnproject/fn/vendor/github.com/gin-gonic/gin/context.go:104 +0x43
github.com/fnproject/fn/api/server.apiMetricsWrap.func1.1(0xc4200c2790)
   /Users/OCliffe/gows/src/github.com/fnproject/fn/api/server/gin_middlewares.go:107 +0x3d4
github.com/fnproject/fn/vendor/github.com/gin-gonic/gin.(*Context).Next(0xc4200c2790)
   /Users/OCliffe/gows/src/github.com/fnproject/fn/vendor/github.com/gin-gonic/gin/context.go:104 +0x43
github.com/fnproject/fn/api/server.panicWrap(0xc4200c2790)
   /Users/OCliffe/gows/src/github.com/fnproject/fn/api/server/gin_middlewares.go:140 +0x51
github.com/fnproject/fn/vendor/github.com/gin-gonic/gin.(*Context).Next(0xc4200c2790)
   /Users/OCliffe/gows/src/github.com/fnproject/fn/vendor/github.com/gin-gonic/gin/context.go:104 +0x43
github.com/fnproject/fn/api/server.traceWrap(0xc4200c2790)
   /Users/OCliffe/gows/src/github.com/fnproject/fn/api/server/gin_middlewares.go:78 +0x3da
github.com/fnproject/fn/vendor/github.com/gin-gonic/gin.(*Context).Next(0xc4200c2790)
   /Users/OCliffe/gows/src/github.com/fnproject/fn/vendor/github.com/gin-gonic/gin/context.go:104 +0x43
github.com/fnproject/fn/api/server.loggerWrap(0xc4200c2790)
   /Users/OCliffe/gows/src/github.com/fnproject/fn/api/server/gin_middlewares.go:157 +0x18e
github.com/fnproject/fn/vendor/github.com/gin-gonic/gin.(*Context).Next(0xc4200c2790)
   /Users/OCliffe/gows/src/github.com/fnproject/fn/vendor/github.com/gin-gonic/gin/context.go:104 +0x43
github.com/fnproject/fn/api/server.withRIDProvider.func1(0xc4200c2790)
   /Users/OCliffe/gows/src/github.com/fnproject/fn/api/server/server_options.go:36 +0x256
github.com/fnproject/fn/vendor/github.com/gin-gonic/gin.(*Context).Next(0xc4200c2790)
   /Users/OCliffe/gows/src/github.com/fnproject/fn/vendor/github.com/gin-gonic/gin/context.go:104 +0x43
github.com/fnproject/fn/vendor/github.com/gin-gonic/gin.(*Engine).handleHTTPRequest(0xc420446480, 0xc4200c2790)
   /Users/OCliffe/gows/src/github.com/fnproject/fn/vendor/github.com/gin-gonic/gin/gin.go:332 +0x585
github.com/fnproject/fn/vendor/github.com/gin-gonic/gin.(*Engine).ServeHTTP(0xc420446480, 0x4e0da40, 0xc420132620, 0xc420172f00)
   /Users/OCliffe/gows/src/github.com/fnproject/fn/vendor/github.com/gin-gonic/gin/gin.go:296 +0x153
github.com/fnproject/fn/vendor/go.opencensus.io/plugin/ochttp.(*Handler).ServeHTTP(0xc4201f2280, 0x4e0da40, 0xc420132620, 0xc420172f00)
   /Users/OCliffe/gows/src/github.com/fnproject/fn/vendor/go.opencensus.io/plugin/ochttp/server.go:75 +0x109
net/http.serverHandler.ServeHTTP(0xc4204fe410, 0x4e0dfc0, 0xc420356c40, 0xc420172e00)
   /usr/local/go/src/net/http/server.go:2694 +0xbc
net/http.(*conn).serve(0xc420377400, 0x4e0efc0, 0xc420471fc0)
   /usr/local/go/src/net/http/server.go:1830 +0x651
created by net/http.(*Server).Serve
   /usr/local/go/src/net/http/server.go:2795 +0x27b
"
time="2018-06-23T19:21:56+01:00" level=error msg="internal server error" error="App not found" stack="