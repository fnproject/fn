package main

import (
	"fmt"
	"net/url"
	"os"

	functions "github.com/iron-io/functions_go"
	"github.com/urfave/cli"
)

const fnversion = "0.1.40"

var aliases = map[string]cli.Command{
	"build":  build(),
	"bump":   bump(),
	"deploy": deploy(),
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

func main() {
	app := cli.NewApp()
	app.Name = "fn"
	app.Version = fnversion
	app.Authors = []cli.Author{{Name: "iron.io"}}
	app.Description = "IronFunctions command line tools"
	app.UsageText = `Check the manual at https://github.com/iron-io/functions/blob/master/fn/README.md`

	cli.AppHelpTemplate = `{{.Name}} {{.Version}}{{if .Description}}

{{.Description}}{{end}}

USAGE:
   {{if .UsageText}}{{.UsageText}}{{else}}{{.HelpName}} {{if .VisibleFlags}}[global options]{{end}}{{if .Commands}} command [command options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}{{end}}

ENVIRONMENT VARIABLES:
   API_URL - IronFunctions remote API address{{if .VisibleCommands}}

COMMANDS:{{range .VisibleCategories}}{{if .Name}}
   {{.Name}}:{{end}}{{range .VisibleCommands}}
     {{join .Names ", "}}{{"\t"}}{{.Usage}}{{end}}{{end}}{{end}}{{if .VisibleFlags}}

ALIASES:
     build    (images build)
     bump     (images bump)
     deploy   (images deploy)

GLOBAL OPTIONS:
   {{range $index, $option := .VisibleFlags}}{{if $index}}
   {{end}}{{$option}}{{end}}{{end}}
`

	app.CommandNotFound = func(c *cli.Context, cmd string) {
		fmt.Fprintf(os.Stderr, "command not found: %v\n", cmd)
	}
	app.Commands = []cli.Command{
		initFn(),
		apps(),
		routes(),
		images(),
		lambda(),
		version(),
	}
	app.Commands = append(app.Commands, aliasesFn()...)
	app.Run(os.Args)
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
