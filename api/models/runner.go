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

package models

import "errors"

var (
	ErrRunnerRouteNotFound   = errors.New("Route not found on that application")
	ErrRunnerInvalidPayload  = errors.New("Invalid payload")
	ErrRunnerRunRoute        = errors.New("Couldn't run this route in the job server")
	ErrRunnerAPICantConnect  = errors.New("Couldn`t connect to the job server API")
	ErrRunnerAPICreateJob    = errors.New("Could not create a job in job server")
	ErrRunnerInvalidResponse = errors.New("Invalid response")
	ErrRunnerTimeout         = errors.New("Timed out")
)
