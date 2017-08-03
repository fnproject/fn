package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/fnproject/fn/cli/client"
	functions "github.com/funcy/functions_go"
	"github.com/onsi/gomega"
	"github.com/urfave/cli"
)

type testStruct struct {
	Tests []fftest `yaml:"tests,omitempty" json:"tests,omitempty"`
}

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
		// cli.BoolFlag{
		// 	Name:        "b",
		// 	Usage:       "build before test",
		// 	Destination: &t.build,
		// },
		cli.StringFlag{
			Name:        "remote",
			Usage:       "run tests by calling the function on Oracle Functions daemon on `appname`",
			Destination: &t.remote,
		},
	}
}

func (t *testcmd) test(c *cli.Context) error {
	gomega.RegisterFailHandler(func(message string, callerSkip ...int) {
		fmt.Println("In gomega FailHandler:", message)
	})

	// First, build it
	err := c.App.Command("build").Run(c)
	if err != nil {
		return err
	}

	ff, err := loadFuncfile()
	if err != nil {
		return err
	}

	var tests []fftest

	// Look for test.json file too
	tfile := "test.json"
	if exists(tfile) {
		f, err := os.Open(tfile)
		if err != nil {
			return fmt.Errorf("could not open %s for parsing. Error: %v", tfile, err)
		}
		ts := &testStruct{}
		err = json.NewDecoder(f).Decode(ts)
		if err != nil {
			fmt.Println("Invalid tests.json file:", err)
			return err
		}
		tests = ts.Tests
	} else {
		tests = ff.Tests
	}
	if len(tests) == 0 {
		return errors.New("no tests found for this function")
	}

	fmt.Printf("Running %v tests...", len(tests))

	target := ff.FullName()
	runtest := runlocaltest
	if t.remote != "" {
		if ff.Path == "" {
			return errors.New("execution of tests on remote server demand that this function has a `path`.")
		}
		if err := resetBasePath(t.Configuration); err != nil {
			return fmt.Errorf("error setting endpoint: %v", err)
		}
		baseURL, err := url.Parse(t.Configuration.BasePath)
		if err != nil {
			return fmt.Errorf("error parsing base path: %v", err)
		}

		u, err := url.Parse("../")
		u.Path = path.Join(u.Path, "r", t.remote, ff.Path)
		target = baseURL.ResolveReference(u).String()
		runtest = runremotetest
	}

	errorCount := 0
	fmt.Println("running tests on", ff.FullName(), ":")
	for i, tt := range tests {
		fmt.Printf("\nTest %v\n", i+1)
		start := time.Now()
		var err error
		err = runtest(target, tt.Input, tt.Output, tt.Err, tt.Env)
		if err != nil {
			fmt.Print("FAILED")
			errorCount += 1
			scanner := bufio.NewScanner(strings.NewReader(err.Error()))
			for scanner.Scan() {
				fmt.Println("\t\t", scanner.Text())
			}
			if err := scanner.Err(); err != nil {
				fmt.Fprintln(os.Stderr, "reading test result:", err)
				break
			}
		} else {
			fmt.Print("PASSED")
		}
		fmt.Println(" - ", tt.Name, " (", time.Since(start), ")")

	}
	fmt.Printf("\n%v tests passed, %v tests failed.\n", len(tests)-errorCount, errorCount)
	if errorCount > 0 {
		return errors.New("tests failed, errors found")
	}
	return nil
}

func runlocaltest(target string, in *inputMap, expectedOut *outputMap, expectedErr *string, env map[string]string) error {
	inBytes, _ := json.Marshal(in.Body)
	stdin := &bytes.Buffer{}
	if in != nil {
		stdin = bytes.NewBuffer(inBytes)
	}
	expectedB, _ := json.Marshal(expectedOut.Body)
	expectedString := string(expectedB)

	// TODO: use the same run as `fn run` so we don't have to dupe all the config and env vars that get passed in
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

	ff := &funcfile{Name: target}
	if err := runff(ff, stdin, &stdout, &stderr, "", restrictedEnv, nil, DefaultFormat, 1); err != nil {
		return fmt.Errorf("%v\nstdout:%s\nstderr:%s\n", err, stdout.String(), stderr.String())
	}

	out := stdout.String()
	if expectedOut == nil && out != "" {
		return fmt.Errorf("unexpected output found: %s", out)
	}
	if gomega.Expect(out).To(gomega.MatchJSON(expectedString)) {
		// PASS!
		return nil
	}

	// don't think we should test error output, it's just for logging
	// err := stderr.String()
	// if expectedErr == nil && err != "" {
	// 	return fmt.Errorf("unexpected error output found: %s", err)
	// } else if expectedErr != nil && *expectedErr != err {
	// 	return fmt.Errorf("mismatched error output found.\nexpected (%d bytes):\n%s\ngot (%d bytes):\n%s\n", len(*expectedErr), *expectedErr, len(err), err)
	// }

	return fmt.Errorf("mismatched output found.\nexpected:\n%s\ngot:\n%s\n", expectedString, out)
}

func runremotetest(target string, in *inputMap, expectedOut *outputMap, expectedErr *string, env map[string]string) error {
	inBytes, _ := json.Marshal(in)
	stdin := &bytes.Buffer{}
	if in != nil {
		stdin = bytes.NewBuffer(inBytes)
	}
	expectedString, _ := json.Marshal(expectedOut.Body)

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
	if err := client.CallFN(target, stdin, &stdout, "", restrictedEnv, false); err != nil {
		return fmt.Errorf("%v\nstdout:%s\n", err, stdout.String())
	}

	out := stdout.String()
	if expectedOut == nil && out != "" {
		return fmt.Errorf("unexpected output found: %s", out)
	}
	if gomega.Expect(out).To(gomega.MatchJSON(expectedString)) {
		// PASS!
		return nil
	}

	return nil
}
