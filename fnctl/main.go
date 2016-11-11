package main

import (
	"fmt"
	"net/url"
	"os"

	functions "github.com/iron-io/functions_go"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "fnctl"
	app.Version = "0.0.1"
	app.Authors = []cli.Author{{Name: "iron.io"}}
	app.Usage = "IronFunctions command line tools"
	app.UsageText = `Check the manual at https://github.com/iron-io/functions/blob/master/fnctl/README.md

ENVIRONMENT VARIABLES:
   API_URL - IronFunctions remote API address`
	app.CommandNotFound = func(c *cli.Context, cmd string) { fmt.Fprintf(os.Stderr, "command not found: %v\n", cmd) }
	app.Commands = []cli.Command{
		apps(),
		build(),
		bump(),
		call(),
		lambda(),
		publish(),
		push(),
		routes(),
		run(),
		initFn(),
	}
	app.Run(os.Args)
}

func resetBasePath(c *functions.Configuration) error {
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080/"
	}

	u, err := url.Parse(apiURL)
	if err != nil {
		return err
	}
	u.Path = "/v1"
	c.BasePath = u.String()

	return nil
}
