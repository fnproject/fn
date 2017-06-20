package main

import (
	"errors"
	"fmt"
	"io"
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
		ArgsUsage: "[username/image:tag]",
		Flags:     append(runflags(), []cli.Flag{}...),
		Action:    r.run,
	}
}

type runCmd struct{}

func runflags() []cli.Flag {
	return []cli.Flag{
		cli.StringSliceFlag{
			Name:  "e",
			Usage: "select environment variables to be sent to function",
		},
		cli.StringSliceFlag{
			Name:  "link",
			Usage: "select container links for the function",
		},
		cli.StringFlag{
			Name:  "method",
			Usage: "http method for function",
		},
	}
}

func (r *runCmd) run(c *cli.Context) error {
	// First, build it
	err := c.App.Command("build").Run(c)
	if err != nil {
		return err
	}
	var ff *funcfile
	// if image name is passed in, it will run that image
	image := c.Args().First()
	if image == "" {
		ff, err = loadFuncfile()
		if err != nil {
			if _, ok := err.(*notFoundError); ok {
				return errors.New("error: image name is missing or no function file found")
			}
			return err
		}
	} else {
		ff = &funcfile{
			Name: image,
		}
	}

	return runff(ff, stdin(), os.Stdout, os.Stderr, c.String("method"), c.StringSlice("e"), c.StringSlice("link"))
}

// TODO: THIS SHOULD USE THE RUNNER DRIVERS FROM THE SERVER SO IT'S ESSENTIALLY THE SAME PROCESS (MINUS DATABASE AND ALL THAT)
func runff(ff *funcfile, stdin io.Reader, stdout, stderr io.Writer, method string, envVars []string, links []string) error {
	sh := []string{"docker", "run", "--rm", "-i"}

	var env []string    // env for the shelled out docker run command
	var runEnv []string // env to pass into the container via -e's

	if method == "" {
		if stdin == nil {
			method = "GET"
		} else {
			method = "POST"
		}
	}
	// Add expected env vars that service will add
	runEnv = append(runEnv, kvEq("METHOD", method))
	runEnv = append(runEnv, kvEq("REQUEST_URL", "http://localhost:8080/myapp/hello"))
	runEnv = append(runEnv, kvEq("APP_NAME", "myapp"))
	runEnv = append(runEnv, kvEq("ROUTE", "/hello")) // TODO: should we change this to PATH ?
	// add user defined envs
	runEnv = append(runEnv, envVars...)

	for _, l := range links {
		sh = append(sh, "--link", l)
	}

	dockerenv := []string{"DOCKER_TLS_VERIFY", "DOCKER_HOST", "DOCKER_CERT_PATH", "DOCKER_MACHINE_NAME"}
	for _, e := range dockerenv {
		env = append(env, fmt.Sprint(e, "=", os.Getenv(e)))
	}

	for _, e := range runEnv {
		sh = append(sh, "-e", e)
	}

	if ff.Type != nil && *ff.Type == "async" {
		// if async, we'll run this in a separate thread and wait for it to complete
		// reqID := id.New().String()
		// I'm starting to think maybe `fn run` locally should work the same whether sync or async?  Or how would we allow to test the output?
	}

	sh = append(sh, ff.FullName())
	cmd := exec.Command(sh[0], sh[1:]...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	// cmd.Env = env
	return cmd.Run()
}

func extractEnvVar(e string) ([]string, string) {
	kv := strings.Split(e, "=")
	name := toEnvName("HEADER", kv[0])
	sh := []string{"-e", name}
	var v string
	if len(kv) > 1 {
		v = kv[1]
	} else {
		v = os.Getenv(kv[0])
	}
	return sh, kvEq(name, v)
}

func kvEq(k, v string) string {
	return fmt.Sprintf("%s=%s", k, v)
}

// From server.toEnvName()
func toEnvName(envtype, name string) string {
	name = strings.ToUpper(strings.Replace(name, "-", "_", -1))
	return fmt.Sprintf("%s_%s", envtype, name)
}
