package server

import (
	"github.com/fnproject/fn/api/runner"
)

// AddRunListeners adds a listener that will be fired before and after a function run.
func (s *Server) AddRunListener(listener runner.RunListener) {
	s.Runner.AddRunListener(listener)
}
