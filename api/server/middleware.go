// TODO: it would be nice to move these into the top level folder so people can use these with the "functions" package, eg: functions.AddMiddleware(...)
package server

import (
	"context"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"gitlab-odx.oracle.com/odx/functions/api/models"
)

// Middleware is the interface required for implementing functions middlewar
type Middleware interface {
	// Serve is what the Middleware must implement. Can modify the request, write output, etc.
	// todo: should we abstract the HTTP out of this?  In case we want to support other protocols.
	Serve(ctx MiddlewareContext, w http.ResponseWriter, r *http.Request, app *models.App) error
}

// MiddlewareFunc func form of Middleware
type MiddlewareFunc func(ctx MiddlewareContext, w http.ResponseWriter, r *http.Request, app *models.App) error

// Serve wrapper
func (f MiddlewareFunc) Serve(ctx MiddlewareContext, w http.ResponseWriter, r *http.Request, app *models.App) error {
	return f(ctx, w, r, app)
}

// MiddlewareContext extends context.Context for Middleware
type MiddlewareContext interface {
	context.Context

	// Set is used to store a new key/value pair exclusively for this context.
	// This is different than WithValue(), as it does not make a copy of the context with the new value, it will be available up the chain as well.
	Set(key string, value interface{})

	// Get returns the value for the given key, ie: (value, true).
	// If the value does not exists it returns (nil, false)
	Get(key string) (value interface{}, exists bool)

	// MustGet returns the value for the given key if it exists, otherwise it panics.
	MustGet(key string) interface{}

	// Middleware can call Next() explicitly to call the next middleware in the chain. If Next() is not called and an error is not returned, Next() will automatically be called.
	Next()
	// Index returns the index of where we're at in the chain
	Index() int
}

type middlewareContextImpl struct {
	context.Context

	ginContext  *gin.Context
	nextCalled  bool
	index       int
	middlewares []Middleware
}

// Set is used to store a new key/value pair exclusively for this context.
// This is different than WithValue(), as it does not make a copy of the context with the new value, it will be available up the chain as well.
func (c *middlewareContextImpl) Set(key string, value interface{}) {
	c.ginContext.Set(key, value)
}

// Get returns the value for the given key, ie: (value, true).
// If the value does not exists it returns (nil, false)
func (c *middlewareContextImpl) Get(key string) (value interface{}, exists bool) {
	return c.ginContext.Get(key)
}

// MustGet returns the value for the given key if it exists, otherwise it panics.
func (c *middlewareContextImpl) MustGet(key string) interface{} {
	return c.ginContext.MustGet(key)
}

func (c *middlewareContextImpl) Next() {
	c.nextCalled = true
	c.index++
	c.serveNext()
}

func (c *middlewareContextImpl) serveNext() {
	if c.Index() >= len(c.middlewares) {
		return
	}
	// make shallow copy:
	fctx2 := *c
	fctx2.nextCalled = false
	r := c.ginContext.Request.WithContext(fctx2)
	err := c.middlewares[c.Index()].Serve(&fctx2, c.ginContext.Writer, r, nil)
	if err != nil {
		logrus.WithError(err).Warnln("Middleware error")
		// todo: might be a good idea to check if anything is written yet, and if not, output the error: simpleError(err)
		// see: http://stackoverflow.com/questions/39415827/golang-http-check-if-responsewriter-has-been-written
		c.ginContext.Error(err)
		c.ginContext.Abort()
		return
	}
	if !fctx2.nextCalled {
		// then we automatically call next
		fctx2.Next()
	}

}

func (c *middlewareContextImpl) Index() int {
	return c.index
}

func (s *Server) middlewareWrapperFunc(ctx context.Context) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: we should get rid of this, gin context and middleware context both implement context, don't need a third one here
		ctx = c.MustGet("ctx").(context.Context)
		fctx := &middlewareContextImpl{Context: ctx}
		// add this context to gin context so we can grab it later
		c.Set("mctx", fctx)
		fctx.index = -1
		fctx.ginContext = c
		fctx.middlewares = s.middlewares
		// start the chain:
		fctx.Next()
	}
}

// AddMiddleware add middleware
func (s *Server) AddMiddleware(m Middleware) {
	s.middlewares = append(s.middlewares, m)
}

// AddMiddlewareFunc adds middleware function
func (s *Server) AddMiddlewareFunc(m func(ctx MiddlewareContext, w http.ResponseWriter, r *http.Request, app *models.App) error) {
	s.AddMiddleware(MiddlewareFunc(m))
}
