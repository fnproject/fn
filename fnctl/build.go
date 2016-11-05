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

package main

import (
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli"
)

func build() cli.Command {
	cmd := buildcmd{commoncmd: &commoncmd{}}
	flags := append([]cli.Flag{}, cmd.flags()...)
	return cli.Command{
		Name:   "build",
		Usage:  "build function version",
		Flags:  flags,
		Action: cmd.scan,
	}
}

type buildcmd struct {
	*commoncmd
}

func (b *buildcmd) scan(c *cli.Context) error {
	b.commoncmd.scan(b.walker)
	return nil
}

func (b *buildcmd) walker(path string, info os.FileInfo, err error, w io.Writer) error {
	walker(path, info, err, w, b.build)
	return nil
}

// build will take the found valid function and build it
func (b *buildcmd) build(path string) error {
	fmt.Fprintln(b.verbwriter, "building", path)
	_, err := b.buildfunc(path)
	return err
}
