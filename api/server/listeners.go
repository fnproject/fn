package server

import "github.com/fnproject/fn/api/extensions"

// AddCallListener adds a listener that will be fired before and after a function is executed.
func (s *Server) AddCallListener(listener extensions.CallListener) {
	s.Agent.AddCallListener(listener)
}
