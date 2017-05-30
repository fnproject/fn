package common

import (
	"gitlab-odx.oracle.com/odx/functions/api/runner/common/stats"
)

// An Environment is a long lived object that carries around 'configuration'
// for the program. Other long-lived objects may embed an environment directly
// into their definition. Environments wrap common functionality like logging
// and metrics. For short-lived request-response like tasks use `Context`,
// which wraps an Environment.

type Environment struct {
	stats.Statter
}

// Initializers are functions that may set up the environment as they like. By default the environment is 'inactive' in the sense that metrics aren't reported.
func NewEnvironment(initializers ...func(e *Environment)) *Environment {
	env := &Environment{&stats.NilStatter{}}
	for _, init := range initializers {
		init(env)
	}
	return env
}
