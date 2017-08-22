package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/urfave/cli"
)

const (
	DefaultFormat = "default"
	HttpFormat    = "http"
	LocalTestURL  = "http://localhost:8080/myapp/hello"
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
			Name:  "env, e",
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
		cli.StringFlag{
			Name:  "format",
			Usage: "format to use. `default` and `http` (hot) formats currently supported.",
		},
		cli.IntFlag{
			Name:  "runs",
			Usage: "for hot functions only, will call the function `runs` times in a row.",
		},
		cli.Uint64Flag{
			Name:  "memory",
			Usage: "RAM to allocate for function, Units: MB",
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

	// means no memory specified through CLI args
	// memory from func.yaml applied
	if c.Uint64("memory") != 0 {
		ff.Memory = c.Uint64("memory")
	}

	return runff(ff, stdin(), os.Stdout, os.Stderr, c.String("method"), c.StringSlice("e"), c.StringSlice("link"), c.String("format"), c.Int("runs"))
}

// TODO: share all this stuff with the Docker driver in server or better yet, actually use the Docker driver
func runff(ff *funcfile, stdin io.Reader, stdout, stderr io.Writer, method string, envVars []string, links []string, format string, runs int) error {
	sh := []string{"docker", "run", "--rm", "-i", fmt.Sprintf("--memory=%dm", ff.Memory)}

	var err error
	var env []string    // env for the shelled out docker run command
	var runEnv []string // env to pass into the container via -e's

	if method == "" {
		if stdin == nil {
			method = "GET"
		} else {
			method = "POST"
		}
	}
	if format == "" {
		format = DefaultFormat
	}
	// Add expected env vars that service will add
	runEnv = append(runEnv, kvEq("METHOD", method))
	runEnv = append(runEnv, kvEq("REQUEST_URL", LocalTestURL))
	runEnv = append(runEnv, kvEq("APP_NAME", "myapp"))
	runEnv = append(runEnv, kvEq("ROUTE", "/hello")) // TODO: should we change this to PATH ?
	runEnv = append(runEnv, kvEq("FN_FORMAT", format))
	runEnv = append(runEnv, kvEq("MEMORY_MB", fmt.Sprintf("%d", ff.Memory)))

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

	if runs <= 0 {
		runs = 1
	}

	if ff.Type != "" && ff.Type == "async" {
		// if async, we'll run this in a separate thread and wait for it to complete
		// reqID := id.New().String()
		// I'm starting to think maybe `fn run` locally should work the same whether sync or async?  Or how would we allow to test the output?
	}
	body := "" // used for hot functions
	if format == HttpFormat {
		// let's swap out stdin for http formatted message
		input := []byte("")
		if stdin != nil {
			input, err = ioutil.ReadAll(stdin)
			if err != nil {
				return fmt.Errorf("error reading from stdin: %v", err)
			}
		}

		var b bytes.Buffer
		for i := 0; i < runs; i++ {
			// making new request each time since Write closes the body
			req, err := http.NewRequest(method, LocalTestURL, strings.NewReader(string(input)))
			if err != nil {
				return fmt.Errorf("error creating http request: %v", err)
			}
			err = req.Write(&b)
			b.Write([]byte("\n"))
		}

		if err != nil {
			return fmt.Errorf("error writing to byte buffer: %v", err)
		}
		body = b.String()
		// fmt.Println("body:", s)
		stdin = strings.NewReader(body)
	}

	sh = append(sh, ff.ImageName())
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
