package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	functions "github.com/iron-io/functions_go"
	"github.com/urfave/cli"
)

func testfn() cli.Command {
	cmd := testcmd{RoutesApi: functions.NewRoutesApi()}
	return cli.Command{
		Name:   "test",
		Usage:  "run functions test if present",
		Flags:  cmd.flags(),
		Action: cmd.test,
	}
}

type testcmd struct {
	*functions.RoutesApi

	build  bool
	remote string
}

func (t *testcmd) flags() []cli.Flag {
	return []cli.Flag{
		cli.BoolFlag{
			Name:        "b",
			Usage:       "build before test",
			Destination: &t.build,
		},
		cli.StringFlag{
			Name:        "remote",
			Usage:       "run tests by calling the function on IronFunctions daemon on `appname`",
			Destination: &t.remote,
		},
	}
}

func (t *testcmd) test(c *cli.Context) error {
	if t.build {
		b := &buildcmd{verbose: true}
		if err := b.build(c); err != nil {
			return err
		}
		fmt.Println()
	}

	ff, err := loadFuncfile()
	if err != nil {
		return err
	}

	if len(ff.Tests) == 0 {
		return errors.New("no tests found for this function")
	}

	target := ff.FullName()
	runtest := runlocaltest
	if t.remote != "" {
		if ff.Path == nil || *ff.Path == "" {
			return errors.New("execution of tests on remote server demand that this function to have a `path`.")
		}
		if err := resetBasePath(t.Configuration); err != nil {
			return fmt.Errorf("error setting endpoint: %v", err)
		}
		baseURL, err := url.Parse(t.Configuration.BasePath)
		if err != nil {
			return fmt.Errorf("error parsing base path: %v", err)
		}

		u, err := url.Parse("../")
		u.Path = path.Join(u.Path, "r", t.remote, *ff.Path)
		target = baseURL.ResolveReference(u).String()
		runtest = runremotetest
	}

	var foundErr bool
	fmt.Println("running tests on", ff.FullName(), ":")
	for _, tt := range ff.Tests {
		start := time.Now()
		var err error
		err = runtest(target, tt.In, tt.Out, tt.Err, tt.Env)

		fmt.Print("\t - ", tt.Name, " (", time.Since(start), "): ")

		if err != nil {
			fmt.Println()
			foundErr = true
			scanner := bufio.NewScanner(strings.NewReader(err.Error()))
			for scanner.Scan() {
				fmt.Println("\t\t", scanner.Text())
			}
			if err := scanner.Err(); err != nil {
				fmt.Fprintln(os.Stderr, "reading test result:", err)
				break
			}
			continue
		}

		fmt.Println("OK")
	}

	if foundErr {
		return errors.New("errors found")
	}
	return nil
}

func runlocaltest(target string, in, expectedOut, expectedErr *string, env map[string]string) error {
	stdin := &bytes.Buffer{}
	if in != nil {
		stdin = bytes.NewBufferString(*in)
	}

	var stdout, stderr bytes.Buffer
	var restrictedEnv []string
	for k, v := range env {
		oldv := os.Getenv(k)
		defer func(oldk, oldv string) {
			os.Setenv(oldk, oldv)
		}(k, oldv)
		os.Setenv(k, v)
		restrictedEnv = append(restrictedEnv, k)
	}

	if err := runff(target, stdin, &stdout, &stderr, "", restrictedEnv, nil); err != nil {
		return fmt.Errorf("%v\nstdout:%s\nstderr:%s\n", err, stdout.String(), stderr.String())
	}

	out := stdout.String()
	if expectedOut == nil && out != "" {
		return fmt.Errorf("unexpected output found: %s", out)
	} else if expectedOut != nil && *expectedOut != out {
		return fmt.Errorf("mismatched output found.\nexpected (%d bytes):\n%s\ngot (%d bytes):\n%s\n", len(*expectedOut), *expectedOut, len(out), out)
	}

	err := stderr.String()
	if expectedErr == nil && err != "" {
		return fmt.Errorf("unexpected error output found: %s", err)
	} else if expectedErr != nil && *expectedErr != err {
		return fmt.Errorf("mismatched error output found.\nexpected (%d bytes):\n%s\ngot (%d bytes):\n%s\n", len(*expectedErr), *expectedErr, len(err), err)
	}

	return nil
}

func runremotetest(target string, in, expectedOut, expectedErr *string, env map[string]string) error {
	stdin := &bytes.Buffer{}
	if in != nil {
		stdin = bytes.NewBufferString(*in)
	}

	var stdout bytes.Buffer

	var restrictedEnv []string
	for k, v := range env {
		oldv := os.Getenv(k)
		defer func(oldk, oldv string) {
			os.Setenv(oldk, oldv)
		}(k, oldv)
		os.Setenv(k, v)
		restrictedEnv = append(restrictedEnv, k)
	}
	if err := callfn(target, stdin, &stdout, "", restrictedEnv); err != nil {
		return fmt.Errorf("%v\nstdout:%s\n", err, stdout.String())
	}

	out := stdout.String()
	if expectedOut == nil && out != "" {
		return fmt.Errorf("unexpected output found: %s", out)
	} else if expectedOut != nil && *expectedOut != out {
		return fmt.Errorf("mismatched output found.\nexpected (%d bytes):\n%s\ngot (%d bytes):\n%s\n", len(*expectedOut), *expectedOut, len(out), out)
	}

	if expectedErr != nil {
		return fmt.Errorf("cannot process stderr in remote calls")
	}

	return nil
}
