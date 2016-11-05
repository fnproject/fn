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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/urfave/cli"
)

func run() cli.Command {
	r := runCmd{}

	return cli.Command{
		Name:      "run",
		Usage:     "run a function locally",
		ArgsUsage: "USERNAME/image:tag",
		Flags:     append(runflags(), []cli.Flag{}...),
		Action:    r.run,
	}
}

type runCmd struct{}

func runflags() []cli.Flag {
	return []cli.Flag{
		cli.StringSliceFlag{
			Name:  "e",
			Usage: "limit the environment variables sent to function, if ommited then all are sent.",
		},
	}
}

func (r *runCmd) run(c *cli.Context) error {
	image := c.Args().First()
	if image == "" {
		return errors.New("error: image name is missing")
	}

	sh := []string{"docker", "run", "--rm", "-i"}

	var env []string
	detectedEnv := os.Environ()
	if se := c.StringSlice("e"); len(se) > 0 {
		detectedEnv = se
	}

	for _, e := range detectedEnv {
		shellvar, envvar := extractEnvVar(e)
		sh = append(sh, shellvar...)
		env = append(env, envvar)
	}

	dockerenv := []string{"DOCKER_TLS_VERIFY", "DOCKER_HOST", "DOCKER_CERT_PATH", "DOCKER_MACHINE_NAME"}
	for _, e := range dockerenv {
		env = append(env, fmt.Sprint(e, "=", os.Getenv(e)))
	}

	sh = append(sh, image)
	cmd := exec.Command(sh[0], sh[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	return cmd.Run()
}

func extractEnvVar(e string) ([]string, string) {
	kv := strings.Split(e, "=")
	name := toEnvName("HEADER", kv[0])
	sh := []string{"-e", name}
	env := fmt.Sprintf("%s=%s", name, os.Getenv(kv[0]))
	return sh, env
}

// From server.toEnvName()
func toEnvName(envtype, name string) string {
	name = strings.ToUpper(strings.Replace(name, "-", "_", -1))
	return fmt.Sprintf("%s_%s", envtype, name)
}
