package main

/*
usage: fnctl init <name>

If there's a Dockerfile found, this will generate the basic file with just the image name. exit
It will then try to decipher the runtime based on the files in the current directory, if it can't figure it out, it will ask.
It will then take a best guess for what the entrypoint will be based on the language, it it can't guess, it will ask.

*/

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"strings"

	"github.com/iron-io/functions/fnctl/langs"
	"github.com/urfave/cli"
)

var (
	fileExtToRuntime = map[string]string{
		".c":     "gcc",
		".class": "java",
		".clj":   "leiningen",
		".cpp":   "gcc",
		".erl":   "erlang",
		".ex":    "elixir",
		".go":    "go",
		".h":     "gcc",
		".java":  "java",
		".js":    "node",
		".php":   "php",
		".pl":    "perl",
		".py":    "python",
		".scala": "scala",
		".rb":    "ruby",
	}

	fnRuntimes []string
)

func init() {
	for rt := range acceptableFnRuntimes {
		fnRuntimes = append(fnRuntimes, rt)
	}
}

type initFnCmd struct {
	name       string
	force      bool
	runtime    *string
	entrypoint *string
}

func initFn() cli.Command {
	a := initFnCmd{}

	return cli.Command{
		Name:        "init",
		Usage:       "create a local function.yaml file",
		Description: "Creates a function.yaml file in the current directory.  ",
		ArgsUsage:   "<DOCKERHUB_USERNAME:FUNCTION_NAME>",
		Action:      a.init,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:        "force, f",
				Usage:       "overwrite existing function.yaml",
				Destination: &a.force,
			},
			cli.StringFlag{
				Name:        "runtime",
				Usage:       "choose an existing runtime - " + strings.Join(fnRuntimes, ", "),
				Destination: a.runtime,
			},
			cli.StringFlag{
				Name:        "entrypoint",
				Usage:       "entrypoint is the command to run to start this function - equivalent to Dockerfile ENTRYPOINT.",
				Destination: a.entrypoint,
			},
		},
	}
}

func (a *initFnCmd) init(c *cli.Context) error {
	if !a.force {
		ff, err := findFuncfile()
		if err != nil {
			if _, ok := err.(*NotFoundError); ok {
				// great, we're about to make one
			} else {
				return err
			}
		}
		if ff != nil {
			return errors.New("function file already exists")
		}
	}

	err := a.buildFuncFile(c)
	if err != nil {
		return err
	}

	/*
	 Now we can make some guesses for the entrypoint based on runtime.
	 If Go, use ./foldername, if ruby, use ruby and a filename. If node, node + filename
	*/

	ff := &funcfile{
		Name:       a.name,
		Runtime:    a.runtime,
		Version:    initialVersion,
		Entrypoint: a.entrypoint,
	}

	if err := encodeFuncfileYAML("function.yaml", ff); err != nil {
		return err
	}
	fmt.Println("function.yaml created.")
	return nil
}

func (a *initFnCmd) buildFuncFile(c *cli.Context) error {
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error detecting current working directory: %s\n", err)
	}

	a.name = c.Args().First()
	if a.name == "" {
		// todo: also check that it's valid image name format
		return errors.New("Please specify a name for your function in the following format <DOCKERHUB_USERNAME>:<FUNCTION_NAME>")
	}

	if exists("Dockerfile") {
		// then don't need anything else
		fmt.Println("Dockerfile found, will use that to build.")
		return nil
	}

	var rt string
	var filename string
	if a.runtime == nil || *a.runtime == "" {
		filename, rt, err = detectRuntime(pwd)
		if err != nil {
			return err
		}
		a.runtime = &rt
		fmt.Printf("assuming %v runtime\n", rt)
	}
	if _, ok := acceptableFnRuntimes[*a.runtime]; !ok {
		return fmt.Errorf("init does not support the %s runtime, you'll have to create your own Dockerfile for this function", *a.runtime)
	}

	if a.entrypoint == nil || *a.entrypoint == "" {
		ep, err := detectEntrypoint(filename, *a.runtime, pwd)
		if err != nil {
			return fmt.Errorf("could not detect entrypoint for %v, use --entrypoint to add it explicitly. %v", *a.runtime, err)
		}
		a.entrypoint = &ep
	}

	return nil
}

// detectRuntime this looks at the files in the directory and if it finds a support file extension, it
// returns the filename and runtime for that extension.
func detectRuntime(path string) (filename string, runtime string, err error) {
	err = filepath.Walk(path, func(_ string, info os.FileInfo, _ error) error {
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(info.Name()))
		if ext == "" {
			return nil
		}
		var ok bool
		runtime, ok = fileExtToRuntime[ext]
		if ok {
			// first match, exiting - http://stackoverflow.com/a/36713726/105562
			filename = info.Name()
			return io.EOF
		}
		return nil
	})
	if err != nil {
		if err == io.EOF {
			return filename, runtime, nil
		}
		return "", "", fmt.Errorf("file walk error: %s\n", err)
	}
	return "", "", fmt.Errorf("no supported files found to guess runtime, please set runtime explicitly with --runtime flag")
}

func detectEntrypoint(filename, runtime, pwd string) (string, error) {
	helper, err := langs.GetLangHelper(runtime)
	if err != nil {
		return "", err
	}
	return helper.Entrypoint(filename)
}

func scoreExtension(path string) (string, error) {
	scores := map[string]uint{
		"": 0,
	}
	err := filepath.Walk(path, func(_ string, info os.FileInfo, _ error) error {
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(info.Name()))
		if ext == "" {
			return nil
		}
		scores[ext]++
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("file walk error: %s\n", err)
	}

	biggest := ""
	for ext, score := range scores {
		if score > scores[biggest] {
			biggest = ext
		}
	}
	return biggest, nil

}
