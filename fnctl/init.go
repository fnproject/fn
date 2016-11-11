package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"strings"

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
	}

	fnRuntimes []string
)

func init() {
	for rt := range acceptableFnRuntimes {
		fnRuntimes = append(fnRuntimes, rt)
	}
}

type initFnCmd struct {
	force   bool
	runtime string
}

func initFn() cli.Command {
	a := initFnCmd{}

	return cli.Command{
		Name:        "init",
		Usage:       "create a local function.yaml file",
		Description: "Entrypoint is the binary file which the container engine will invoke when the request comes in - equivalent to Dockerfile ENTRYPOINT.",
		ArgsUsage:   "<entrypoint>",
		Action:      a.init,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:        "f",
				Usage:       "overwrite existing function.yaml",
				Destination: &a.force,
			},
			cli.StringFlag{
				Name:        "runtime",
				Usage:       "choose an existing runtime - " + strings.Join(fnRuntimes, ", "),
				Destination: &a.runtime,
			},
		},
	}
}

func (a *initFnCmd) init(c *cli.Context) error {
	if !a.force {
		for _, fn := range validfn {
			if _, err := os.Stat(fn); !os.IsNotExist(err) {
				return errors.New("function file already exists")
			}
		}
	}

	entrypoint := c.Args().First()
	if entrypoint == "" {
		fmt.Print("Please, specify an entrypoint for your function: ")
		fmt.Scanln(&entrypoint)
	}
	if entrypoint == "" {
		return errors.New("entrypoint is missing")
	}

	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error detecting current working directory: %s\n", err)
	}

	if a.runtime == "" {
		rt, err := detectRuntime(pwd)
		if err != nil {
			return err
		}
		var ok bool
		a.runtime, ok = fileExtToRuntime[rt]
		if !ok {
			return fmt.Errorf("could not detect language of this function: %s\n", a.runtime)
		}
	}

	if _, ok := acceptableFnRuntimes[a.runtime]; !ok {
		return fmt.Errorf("cannot use runtime %s", a.runtime)
	}

	ff := &funcfile{
		Runtime:    &a.runtime,
		Version:    initialVersion,
		Entrypoint: &entrypoint,
	}

	if err := encodeFuncfileYAML("function.yaml", ff); err != nil {
		return err
	}
	fmt.Println("function.yaml written")
	return nil
}

func detectRuntime(path string) (string, error) {
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
