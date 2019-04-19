package server

import (
	"net/http"

	"github.com/fnproject/fn/fnext"
	"github.com/gin-gonic/gin"
)

func (s *Server) apiHandlerWrapperFn(apiHandler fnext.APIHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiHandler.ServeHTTP(c.Writer, c.Request)
	}
}

// AddEndpoint adds an endpoint to /v2/x
func (s *Server) AddEndpoint(method, path string, handler fnext.APIHandler) {
	v2 := s.Router.Group("/v2")
	v2.Handle(method, path, s.apiHandlerWrapperFn(handler))
}

// AddEndpointFunc adds an endpoint to /v2/x
func (s *Server) AddEndpointFunc(method, path string, handler func(w http.ResponseWriter, r *http.Request)) {
	s.AddEndpoint(method, path, fnext.APIHandlerFunc(handler))
}
