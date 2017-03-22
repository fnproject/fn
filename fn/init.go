package main

/*
usage: fn init <name>

If there's a Dockerfile found, this will generate the basic file with just the image name. exit
It will then try to decipher the runtime based on the files in the current directory, if it can't figure it out, it will ask.
It will then take a best guess for what the entrypoint will be based on the language, it it can't guess, it will ask.

*/

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"strings"

	"github.com/iron-io/functions/fn/langs"
	"github.com/urfave/cli"
)

var (
	fileExtToRuntime = map[string]string{
		".go": "go",
		".js": "node",
		".rb": "ruby",
		".py": "python",
		".rs": "rust",
		".cs": "dotnet",
		".fs": "dotnet",
	}

	fnInitRuntimes []string
)

func init() {
	for rt := range fileExtToRuntime {
		fnInitRuntimes = append(fnInitRuntimes, rt)
	}
}

type initFnCmd struct {
	name           string
	force          bool
	runtime        string
	entrypoint     string
	cmd            string
	format         string
	maxConcurrency int
}

func initFn() cli.Command {
	a := initFnCmd{}

	return cli.Command{
		Name:        "init",
		Usage:       "create a local func.yaml file",
		Description: "Creates a func.yaml file in the current directory.  ",
		ArgsUsage:   "<DOCKERHUB_USERNAME/FUNCTION_NAME>",
		Action:      a.init,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:        "force, f",
				Usage:       "overwrite existing func.yaml",
				Destination: &a.force,
			},
			cli.StringFlag{
				Name:        "runtime",
				Usage:       "choose an existing runtime - " + strings.Join(fnInitRuntimes, ", "),
				Destination: &a.runtime,
			},
			cli.StringFlag{
				Name:        "entrypoint",
				Usage:       "entrypoint is the command to run to start this function - equivalent to Dockerfile ENTRYPOINT.",
				Destination: &a.entrypoint,
			},
			cli.StringFlag{
				Name:        "format",
				Usage:       "hot function IO format - json or http",
				Destination: &a.format,
				Value:       "",
			},
			cli.IntFlag{
				Name:        "max-concurrency",
				Usage:       "maximum concurrency for hot function",
				Destination: &a.maxConcurrency,
				Value:       1,
			},
		},
	}
}

func (a *initFnCmd) init(c *cli.Context) error {
	if !a.force {
		ff, err := loadFuncfile()
		if _, ok := err.(*notFoundError); !ok && err != nil {
			return err
		}
		if ff != nil {
			return errors.New("function file already exists")
		}
	}

	err := a.buildFuncFile(c)
	if err != nil {
		return err
	}

	var ffmt *string
	if a.format != "" {
		ffmt = &a.format
	}

	ff := &funcfile{
		Name:           a.name,
		Runtime:        &a.runtime,
		Version:        initialVersion,
		Entrypoint:     a.entrypoint,
		Cmd:            a.cmd,
		Format:         ffmt,
		MaxConcurrency: &a.maxConcurrency,
	}

	_, path := appNamePath(ff.FullName())
	ff.Path = &path

	if err := encodeFuncfileYAML("func.yaml", ff); err != nil {
		return err
	}

	fmt.Println("func.yaml created.")
	return nil
}

func (a *initFnCmd) buildFuncFile(c *cli.Context) error {
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error detecting current working directory: %s", err)
	}

	a.name = c.Args().First()
	if a.name == "" || strings.Contains(a.name, ":") {
		return errors.New("Please specify a name for your function in the following format <DOCKERHUB_USERNAME>/<FUNCTION_NAME>.\nTry: fn init <DOCKERHUB_USERNAME>/<FUNCTION_NAME>")
	}

	if exists("Dockerfile") {
		fmt.Println("Dockerfile found, will use that to build.")
		return nil
	}

	var rt string
	if a.runtime == "" {
		rt, err = detectRuntime(pwd)
		if err != nil {
			return err
		}
		a.runtime = rt
		fmt.Printf("assuming %v runtime\n", rt)
	}
	fmt.Println("runtime:", a.runtime)
	if _, ok := acceptableFnRuntimes[a.runtime]; !ok {
		return fmt.Errorf("init does not support the %s runtime, you'll have to create your own Dockerfile for this function", a.runtime)
	}

	helper := langs.GetLangHelper(a.runtime)
	if helper == nil {
		fmt.Printf("No helper found for %s runtime, you'll have to pass in the appropriate flags or use a Dockerfile.", a.runtime)
	}

	if a.entrypoint == "" {
		if helper != nil {
			a.entrypoint = helper.Entrypoint()
		}
	}
	if a.cmd == "" {
		if helper != nil {
			a.cmd = helper.Cmd()
		}
	}
	if a.entrypoint == "" && a.cmd == "" {
		return fmt.Errorf("could not detect entrypoint or cmd for %v, use --entrypoint and/or --cmd to set them explicitly", a.runtime)
	}

	return nil
}

func detectRuntime(path string) (runtime string, err error) {
	for ext, runtime := range fileExtToRuntime {
		fn := filepath.Join(path, fmt.Sprintf("func%s", ext))
		if exists(fn) {
			return runtime, nil
		}
	}
	return "", fmt.Errorf("no supported files found to guess runtime, please set runtime explicitly with --runtime flag")
}
