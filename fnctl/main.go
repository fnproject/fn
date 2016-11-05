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
	"net/url"
	"os"

	functions "github.com/iron-io/functions_go"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "fnctl"
	app.Version = "0.0.1"
	app.Authors = []cli.Author{{Name: "iron.io"}}
	app.Usage = "IronFunctions command line tools"
	app.UsageText = "Check the manual at https://github.com/iron-io/functions/blob/master/fnctl/README.md"
	app.CommandNotFound = func(c *cli.Context, cmd string) { fmt.Fprintf(os.Stderr, "command not found: %v\n", cmd) }
	app.Commands = []cli.Command{
		apps(),
		build(),
		bump(),
		call(),
		lambda(),
		publish(),
		routes(),
		run(),
	}
	app.Run(os.Args)
}

func resetBasePath(c *functions.Configuration) {
	var u url.URL
	u.Scheme = c.Scheme
	u.Host = c.Host
	u.Path = "/v1"
	c.BasePath = u.String()
}

func confFlags(c *functions.Configuration) []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{
			Name:        "host",
			Usage:       "raw host path to functions api, e.g. functions.iron.io",
			Destination: &c.Host,
			EnvVar:      "HOST",
			Value:       "localhost:8080",
		},
		cli.StringFlag{
			Name:        "scheme",
			Usage:       "http/https",
			Destination: &c.Scheme,
			EnvVar:      "SCHEME",
			Value:       "http",
		},
	}
}
