package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ServerOption func(context.Context, *Server) error

func EnableShutdownEndpoint(ctx context.Context, halt context.CancelFunc) ServerOption {
	return func(ctx context.Context, s *Server) error {
		s.Router.GET("/shutdown", s.handleShutdown(halt))
		return nil
	}
}

func LimitRequestBody(max int64) ServerOption {
	return func(ctx context.Context, s *Server) error {
		s.Router.Use(limitRequestBody(max))
		return nil
	}
}

func limitRequestBody(max int64) func(c *gin.Context) {
	return func(c *gin.Context) {
		cl := int64(c.Request.ContentLength)
		if cl > max {
			// try to deny this quickly, instead of just letting it get lopped off

			handleErrorResponse(c, errTooBig{cl, max})
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
