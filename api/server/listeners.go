package server

import "github.com/fnproject/fn/api/extenders"

// AddCallListener adds a listener that will be fired before and after a function is executed.
func (s *Server) AddCallListener(listener extenders.CallListener) {
	s.Agent.AddCallListener(listener)
}
