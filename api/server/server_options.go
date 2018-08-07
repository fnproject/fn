package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/fnproject/fn/api/common"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Option is a func that allows configuring a Server
type Option func(context.Context, *Server) error

// RIDProvider is used to generate request IDs
type RIDProvider struct {
	HeaderName   string              // The name of the header where the reques id is stored in the incoming request
	RIDGenerator func(string) string // Function to generate the requestID
}

// WithRIDProvider will generate request ids for each http request using the
// given generator.
func WithRIDProvider(ridProvider *RIDProvider) Option {
	return func(ctx context.Context, s *Server) error {
		s.Router.Use(withRIDProvider(ridProvider))
		return nil
	}
}

func withRIDProvider(ridp *RIDProvider) func(c *gin.Context) {
	return func(c *gin.Context) {
		rid := ridp.RIDGenerator(c.Request.Header.Get(ridp.HeaderName))
		ctx := common.WithRequestID(c.Request.Context(), rid)
		// We set the rid in the common logger so it is always logged when the common logger is used
		l := common.Logger(ctx).WithFields(logrus.Fields{common.RequestIDContextKey: rid})
		ctx = common.WithLogger(ctx, l)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// EnableShutdownEndpoint adds /shutdown to initiate a shutdown of an fn server.
func EnableShutdownEndpoint(ctx context.Context, halt context.CancelFunc) Option {
	return func(ctx context.Context, s *Server) error {
		s.Router.GET("/shutdown", s.handleShutdown(halt))
		return nil
	}
}

// LimitRequestBody wraps every http request to limit its size to the specified max bytes.
func LimitRequestBody(max int64) Option {
	return func(ctx context.Context, s *Server) error {
		if max > 0 {
			s.Router.Use(limitRequestBody(max))
		}
		return nil
	}
}

func limitRequestBody(max int64) func(c *gin.Context) {
	return func(c *gin.Context) {
		cl := int64(c.Request.ContentLength)
		if cl > max {
			// try to deny this quickly, instead of just letting it get lopped off

			handleV1ErrorResponse(c, errTooBig{cl, max})
			c.Abort()
			return
		}

		// if no Content-Length specified, limit how many bytes we read and error
		// if we hit the max (intercontinental anti-air missile defense system).
		// read http.MaxBytesReader for gritty details..
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max)
		c.Next()
	}
}

// models.APIError
type errTooBig struct {
	n, max int64
}

func (e errTooBig) Code() int { return http.StatusRequestEntityTooLarge }
func (e errTooBig) Error() string {
	return fmt.Sprintf("Content-Length too large for this server, %d > max %d", e.n, e.max)
}
