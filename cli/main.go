package main

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"strings"

	functions "github.com/funcy/functions_go"
	"github.com/urfave/cli"
)

var aliases = map[string]cli.Command{
	"build":  build(),
	"bump":   bump(),
	"deploy": deploy(),
	"push":   push(),
	"run":    run(),
	"call":   call(),
	"calls":  calls(),
	"logs":   logs(),
}

func aliasesFn() []cli.Command {
	cmds := []cli.Command{}
	for alias, cmd := range aliases {
		cmd.Name = alias
		cmd.Hidden = true
		cmds = append(cmds, cmd)
	}
	return cmds
}

func newFn() *cli.App {
	app := cli.NewApp()
	app.Name = "fn"
	app.Version = Version
	app.Authors = []cli.Author{{Name: "Oracle Corporation"}}
	app.Description = "Fn command line tool"
	app.UsageText = `Check the docs at https://github.com/fnproject/fn/blob/master/fn/README.md`

	cli.AppHelpTemplate = `{{.Name}} {{.Version}}{{if .Description}}

{{.Description}}{{end}}

USAGE:
   {{if .UsageText}}{{.UsageText}}{{else}}{{.HelpName}} {{if .VisibleFlags}}[global options]{{end}}{{if .Commands}} command [command options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}{{end}}

ENVIRONMENT VARIABLES:
   API_URL - Oracle Functions remote API address{{if .VisibleCommands}}

COMMANDS:{{range .VisibleCategories}}{{if .Name}}
   {{.Name}}:{{end}}{{range .VisibleCommands}}
     {{join .Names ", "}}{{"\t"}}{{.Usage}}{{end}}{{end}}{{end}}{{if .VisibleFlags}}

ALIASES:
     build    (images build)
     bump     (images bump)
     deploy   (images deploy)
     run      (images run)
     call     (routes call)
     push     (images push)

GLOBAL OPTIONS:
   {{range $index, $option := .VisibleFlags}}{{if $index}}
   {{end}}{{$option}}{{end}}{{end}}
`

	app.CommandNotFound = func(c *cli.Context, cmd string) {
		fmt.Fprintf(os.Stderr, "command not found: %v\n", cmd)
	}
	app.Commands = []cli.Command{
		startCmd(),
		updateCmd(),
		initFn(),
		apps(),
		routes(),
		images(),
		lambda(),
		version(),
		calls(),
		logs(),
		testfn(),
	}
	app.Commands = append(app.Commands, aliasesFn()...)

	prepareCmdArgsValidation(app.Commands)

	return app
}

func parseArgs(c *cli.Context) ([]string, []string) {
	args := strings.Split(c.Command.ArgsUsage, " ")
	var reqArgs []string
	var optArgs []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "[") {
			optArgs = append(optArgs, arg)
		} else if strings.Trim(arg, " ") != "" {
			reqArgs = append(reqArgs, arg)
		}
	}
	return reqArgs, optArgs
}

func prepareCmdArgsValidation(cmds []cli.Command) {
	// TODO: refactor fn to use urfave/cli.v2
	// v1 doesn't let us validate args before the cmd.Action

	for i, cmd := range cmds {
		prepareCmdArgsValidation(cmd.Subcommands)
		if cmd.Action == nil {
			continue
		}
		action := cmd.Action
		cmd.Action = func(c *cli.Context) error {
			reqArgs, _ := parseArgs(c)
			if c.NArg() < len(reqArgs) {
				var help bytes.Buffer
				cli.HelpPrinter(&help, cli.CommandHelpTemplate, c.Command)
				return fmt.Errorf("ERROR: Missing required arguments: %s\n\n%s", strings.Join(reqArgs[c.NArg():], " "), help.String())
			}
			return cli.HandleAction(action, c)
		}
		cmds[i] = cmd
	}
}

func main() {
	app := newFn()
	err := app.Run(os.Args)
	if err != nil {
		// TODO: this doesn't seem to get called even when an error returns from a command, but maybe urfave is doing a non zero exit anyways? nope: https://github.com/urfave/cli/issues/610
		fmt.Printf("Error occurred: %v, exiting...\n", err)
		os.Exit(1)
	}
}

func resetBasePath(c *functions.Configuration) error {
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}

	u, err := url.Parse(apiURL)
	if err != nil {
		return err
	}
	u.Path = "/v1"
	c.BasePath = u.String()

	return nil
}
