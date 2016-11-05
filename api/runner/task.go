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

package runner

import (
	"context"
	"io"

	"github.com/fsouza/go-dockerclient"
	"github.com/iron-io/runner/drivers"
)

type containerTask struct {
	ctx    context.Context
	cfg    *Config
	canRun chan bool
}

func (t *containerTask) Command() string { return "" }

func (t *containerTask) EnvVars() map[string]string {
	return t.cfg.Env
}
func (t *containerTask) Input() io.Reader {
	return t.cfg.Stdin
}

func (t *containerTask) Labels() map[string]string {
	return map[string]string{
		"LogName": t.cfg.AppName,
	}
}

func (t *containerTask) Id() string                         { return t.cfg.ID }
func (t *containerTask) Route() string                      { return "" }
func (t *containerTask) Image() string                      { return t.cfg.Image }
func (t *containerTask) Timeout() uint                      { return uint(t.cfg.Timeout.Seconds()) }
func (t *containerTask) Logger() (stdout, stderr io.Writer) { return t.cfg.Stdout, t.cfg.Stderr }
func (t *containerTask) Volumes() [][2]string               { return [][2]string{} }
func (t *containerTask) WorkDir() string                    { return "" }

func (t *containerTask) Close()                 {}
func (t *containerTask) WriteStat(drivers.Stat) {}

// FIXME: for now just use empty creds => public docker hub image
func (t *containerTask) DockerAuth() docker.AuthConfiguration { return docker.AuthConfiguration{} }
