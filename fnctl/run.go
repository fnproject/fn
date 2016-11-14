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
		// check for a funcfile
		ff, err := findFuncfile()
		if err != nil {
			if _, ok := err.(*NotFoundError); ok {
				return errors.New("error: image name is missing or no function file found")
			} else {
				return err
			}
		}
		image = ff.FullName()
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
	// Check if stdin is being piped, and if not, create our own pipe with nothing in it
	// http://stackoverflow.com/questions/22744443/check-if-there-is-something-to-read-on-stdin-in-golang
	stat, err := os.Stdin.Stat()
	if err != nil {
		// On Windows, this gets an error if nothing is piped in.
		// If something is piped in, it works fine.
		// Turns out, this works just fine in our case as the piped stuff works properly and the non-piped doesn't hang either.
		// See: https://github.com/golang/go/issues/14853#issuecomment-260170423
		// log.Println("Warning: couldn't stat stdin, you are probably on Windows. Be sure to pipe something into this command, eg: 'echo \"hello\" | fnctl run'")
	} else {
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			// log.Println("data is being piped to stdin")
			cmd.Stdin = os.Stdin
		} else {
			// log.Println("stdin is from a terminal")
			cmd.Stdin = strings.NewReader("")
		}
	}
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
