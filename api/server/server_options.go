package server

import "context"

type ServerOption func(*Server)

func EnableShutdownEndpoint(halt context.CancelFunc) ServerOption {
	return func(s *Server) {
		s.Router.GET("/shutdown", s.handleShutdown(halt))
	}
}
