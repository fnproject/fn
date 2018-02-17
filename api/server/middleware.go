package server

import (
	"context"
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/fnext"
	"github.com/gin-gonic/gin"
)

type middlewareController struct {
	// NOTE: I tried to make this work as if it were a normal context, but it just doesn't work right. If someone
	// does something like context.WithValue, then the return is a new &context.valueCtx{} which can't be cast. So now stuffing it into a value instead.
	// context.Context

	// separating this out so we can use it and don't have to reimplement context.Context above
	ginContext     *gin.Context
	server         *Server
	functionCalled bool
}

// CallFunction bypasses any further gin routing and calls the function directly
func (c *middlewareController) CallFunction(w http.ResponseWriter, r *http.Request) {
	c.functionCalled = true
	ctx := r.Context()

	ctx = context.WithValue(ctx, fnext.MiddlewareControllerKey, c)
	r = r.WithContext(ctx)
	c.ginContext.Request = r

	// since we added middleware that checks the app ID
	// we need to ensure that we set it as soon as possible
	appName := ctx.Value(api.CApp).(string)
	if appName != "" {
		appID, err := c.server.datastore.GetAppID(ctx, appName)
		if err != nil {
			handleErrorResponse(c.ginContext, err)
			c.ginContext.Abort()
			return
		}
		c.ginContext.Set(api.AppID, appID)
	}

	c.server.handleFunctionCall(c.ginContext)
	c.ginContext.Abort()
}
func (c *middlewareController) FunctionCalled() bool {
	return c.functionCalled
}

func (s *Server) apiMiddlewareWrapper() gin.HandlerFunc {
	return func(c *gin.Context) {
		// fmt.Println("api middleware")
		s.runMiddleware(c, s.apiMiddlewares)
	}
}

func (s *Server) rootMiddlewareWrapper() gin.HandlerFunc {
	return func(c *gin.Context) {
		// fmt.Println("ROOT MIDDLE")
		s.runMiddleware(c, s.rootMiddlewares)
	}
}

// This is basically a single gin middleware that runs a bunch of fn middleware.
// The final handler will pass it back to gin for further processing.
func (s *Server) runMiddleware(c *gin.Context, ms []fnext.Middleware) {
	// fmt.Println("runMiddleware")
	if len(ms) == 0 {
		// fmt.Println("ZERO MIDDLEWARE")
		c.Next()
		return
	}
	defer func() {
		//This is so that if the server errors or panics on a middleware the server will still respond and not send eof to client.
		err := recover()
		if err != nil {
			common.Logger(c.Request.Context()).WithField("MiddleWarePanicRecovery:", err).Errorln("A panic occurred during middleware.")
			handleErrorResponse(c, ErrInternalServerError)
		}
	}()

	ctx := context.WithValue(c.Request.Context(), fnext.MiddlewareControllerKey, s.newMiddlewareController(c))
	last := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// fmt.Println("final handler called")
		ctx := r.Context()
		mctx := fnext.GetMiddlewareController(ctx)
		// check for bypass
		if mctx.FunctionCalled() {
			// fmt.Println("func already called, skipping")
			c.Abort()
			return
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})

	chainAndServe(ms, c.Writer, c.Request.WithContext(ctx), last)

	c.Abort() // we always abort here because the middleware decides to call next or not. If the `last` handler gets called, it will continue, otherwise we abort.
}

func (s *Server) newMiddlewareController(c *gin.Context) *middlewareController {
	return &middlewareController{
		ginContext: c,
		server:     s,
	}
}

// TODO: I will remove this and other debug commented lines once I'm sure it's all right.
func debugH(i int, m fnext.Middleware, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// fmt.Println("handling", i, "m:", reflect.TypeOf(m), "h:", reflect.TypeOf(h))
		h.ServeHTTP(w, r) // call original
	})
}

// chainAndServe essentially makes a chain of middleware wrapped around each other, then calls ServerHTTP on the end result.
// then each middleware also calls ServeHTTP within it
func chainAndServe(ms []fnext.Middleware, w http.ResponseWriter, r *http.Request, last http.Handler) {
	h := last
	// These get chained in reverse order so they play out in the right order. Don't ask.
	for i := len(ms) - 1; i >= 0; i-- {
		m := ms[i]
		h = m.Handle(debugH(i, m, h))
	}
	h.ServeHTTP(w, r)
}

// AddMiddleware DEPRECATED - see AddAPIMiddleware
func (s *Server) AddMiddleware(m fnext.Middleware) {
	s.AddAPIMiddleware(m)
}

// AddMiddlewareFunc DEPRECATED - see AddAPIMiddlewareFunc
func (s *Server) AddMiddlewareFunc(m fnext.MiddlewareFunc) {
	s.AddAPIMiddlewareFunc(m)
}

// AddAPIMiddleware add middleware
func (s *Server) AddAPIMiddleware(m fnext.Middleware) {
	s.apiMiddlewares = append(s.apiMiddlewares, m)
}

// AddAPIMiddlewareFunc add middlewarefunc
func (s *Server) AddAPIMiddlewareFunc(m fnext.MiddlewareFunc) {
	s.AddAPIMiddleware(m)
}

// AddRootMiddleware add middleware add middleware for end user applications
func (s *Server) AddRootMiddleware(m fnext.Middleware) {
	s.rootMiddlewares = append(s.rootMiddlewares, m)
}

// AddRootMiddlewareFunc add middleware for end user applications
func (s *Server) AddRootMiddlewareFunc(m fnext.MiddlewareFunc) {
	s.AddRootMiddleware(m)
}
