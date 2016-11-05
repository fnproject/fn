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

import "github.com/go-openapi/errors"

type RouteWrapper struct {
	Route *Route `json:"route"`
}

func (m *RouteWrapper) Validate() error {
	var res []error

	if err := m.validateRoute(); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *RouteWrapper) validateRoute() error {

	if m.Route != nil {
		if err := m.Route.Validate(); err != nil {
			return err
		}
	}

	return nil
}
