// Copyright 2016 Iron.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"github.com/treeder/functions/api/runner/common/stats"
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
