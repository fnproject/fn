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

package mock

import (
	"context"
	"fmt"

	"gitlab.oracledx.com/odx/functions/api/runner/drivers"
)

func New() drivers.Driver {
	return &Mocker{}
}

type Mocker struct {
	count int
}

func (m *Mocker) Prepare(context.Context, drivers.ContainerTask) (drivers.Cookie, error) {
	return &cookie{m}, nil
}

type cookie struct {
	m *Mocker
}

func (c *cookie) Close() error { return nil }

func (c *cookie) Run(ctx context.Context) (drivers.RunResult, error) {
	c.m.count++
	if c.m.count%100 == 0 {
		return nil, fmt.Errorf("Mocker error! Bad.")
	}
	return &runResult{
		error:       nil,
		StatusValue: "success",
	}, nil
}

type runResult struct {
	error
	StatusValue string
}

func (runResult *runResult) Status() string {
	return runResult.StatusValue
}
