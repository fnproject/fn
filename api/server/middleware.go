package server

import (
	"context"
	"net/http"

	"gitlab-odx.oracle.com/odx/functions/api/runner/common"

	"github.com/gin-gonic/gin"
)

// Middleware just takes a http.Handler and returns one. So the next middle ware must be called
// within the returned handler or it would be ignored.
type Middleware interface {
	Chain(next http.Handler) http.Handler
}

// MiddlewareFunc is a here to allow a plain function to be a middleware.
type MiddlewareFunc func(next http.Handler) http.Handler

// Chain used to allow middlewarefuncs to be middleware.
func (m MiddlewareFunc) Chain(next http.Handler) http.Handler {
	return m(next)
}

func (s *Server) middlewareWrapperFunc(ctx context.Context) gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(s.middlewares) > 0 {
			defer func() {
				//This is so that if the server errors or panics on a middleware the server will still respond and not send eof to client.
				err := recover()
				if err != nil {
					common.Logger(c.Request.Context()).WithField("MiddleWarePanicRecovery:", err).Errorln("A panic occurred during middleware.")
					handleErrorResponse(c, ErrInternalServerError)
				}
			}()
			var h http.Handler
			keepgoing := false
			h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				c.Request = c.Request.WithContext(r.Context())
				keepgoing = true
			})

			s.chainAndServe(c.Writer, c.Request, h)
			if !keepgoing {
				c.Abort()
			}
		}
	}
}

func (s *Server) chainAndServe(w http.ResponseWriter, r *http.Request, h http.Handler) {
	for _, m := range s.middlewares {
		h = m.Chain(h)
	}
	h.ServeHTTP(w, r)
}

// AddMiddleware add middleware
func (s *Server) AddMiddleware(m Middleware) {
	//Prepend to array so that we can do first,second,third,last,third,second,first
	//and not third,second,first,last,first,second,third
	s.middlewares = append([]Middleware{m}, s.middlewares...)
}

// AddMiddlewareFunc add middlewarefunc
func (s *Server) AddMiddlewareFunc(m MiddlewareFunc) {
	s.AddMiddleware(m)
}
