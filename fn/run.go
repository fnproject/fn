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
			Usage: "select environment variables to be sent to function",
		},
		cli.StringSliceFlag{
			Name:  "link",
			Usage: "select container links for the function",
		},
	}
}

func (r *runCmd) run(c *cli.Context) error {
	image := c.Args().First()
	if image == "" {
		ff, err := loadFuncfile()
		if err != nil {
			if _, ok := err.(*notFoundError); ok {
				return errors.New("error: image name is missing or no function file found")
			}
			return err
		}
		image = ff.FullName()
	}

	return runff(image, stdin(), os.Stdout, os.Stderr, c.StringSlice("e"), c.StringSlice("link"))
}

func runff(image string, stdin io.Reader, stdout, stderr io.Writer, restrictedEnv []string, links []string) error {
	sh := []string{"docker", "run", "--rm", "-i"}

	var env []string
	detectedEnv := os.Environ()
	if len(restrictedEnv) > 0 {
		detectedEnv = restrictedEnv
	}

	for _, e := range detectedEnv {
		shellvar, envvar := extractEnvVar(e)
		sh = append(sh, shellvar...)
		env = append(env, envvar)
	}

	for _, l := range links {
		sh = append(sh, "--link", l)
	}

	dockerenv := []string{"DOCKER_TLS_VERIFY", "DOCKER_HOST", "DOCKER_CERT_PATH", "DOCKER_MACHINE_NAME"}
	for _, e := range dockerenv {
		env = append(env, fmt.Sprint(e, "=", os.Getenv(e)))
	}

	sh = append(sh, image)
	cmd := exec.Command(sh[0], sh[1:]...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = env
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
	env := fmt.Sprintf("%s=%s", name, v)
	return sh, env
}

// From server.toEnvName()
func toEnvName(envtype, name string) string {
	name = strings.ToUpper(strings.Replace(name, "-", "_", -1))
	return fmt.Sprintf("%s_%s", envtype, name)
}
