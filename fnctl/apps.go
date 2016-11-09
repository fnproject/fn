package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/iron-io/functions_go"
	"github.com/urfave/cli"
)

type appsCmd struct {
	*functions.AppsApi
}

func apps() cli.Command {
	a := appsCmd{AppsApi: functions.NewAppsApi()}

	return cli.Command{
		Name:      "apps",
		Usage:     "list apps",
		ArgsUsage: "fnctl apps",
		Flags:     append(confFlags(&a.Configuration), []cli.Flag{}...),
		Action:    a.list,
		Subcommands: []cli.Command{
			{
				Name:   "create",
				Usage:  "create a new app",
				Action: a.create,
				Flags: []cli.Flag{
					cli.StringSliceFlag{
						Name:  "config",
						Usage: "application configuration",
					},
				},
			},
			{
				Name:   "describe",
				Usage:  "describe an existing app",
				Action: a.describe,
			},
		},
	}
}

func (a *appsCmd) list(c *cli.Context) error {
	if err := resetBasePath(&a.Configuration); err != nil {
		return fmt.Errorf("error setting endpoint: %v", err)
	}

	wrapper, _, err := a.AppsGet()
	if err != nil {
		return fmt.Errorf("error getting app: %v", err)
	}

	if len(wrapper.Apps) == 0 {
		fmt.Println("no apps found")
		return nil
	}

	for _, app := range wrapper.Apps {
		fmt.Println(app.Name)
	}

	return nil
}

func (a *appsCmd) create(c *cli.Context) error {
	if c.Args().First() == "" {
		return errors.New("error: app creating takes one argument, an app name")
	}

	if err := resetBasePath(&a.Configuration); err != nil {
		return fmt.Errorf("error setting endpoint: %v", err)
	}

	appName := c.Args().Get(0)
	configs := make(map[string]string)
	for _, v := range c.StringSlice("config") {
		kv := strings.SplitN(v, "=", 2)
		configs[kv[0]] = kv[1]
	}
	body := functions.AppWrapper{App: functions.App{
		Name:   appName,
		Config: configs,
	}}
	wrapper, _, err := a.AppsPost(body)
	if err != nil {
		return fmt.Errorf("error creating app: %v", err)
	}

	fmt.Println(wrapper.App.Name, "created")
	return nil
}

func (a *appsCmd) describe(c *cli.Context) error {
	if c.Args().First() == "" {
		return errors.New("error: app description takes one argument, an app name")
	}

	if err := resetBasePath(&a.Configuration); err != nil {
		return fmt.Errorf("error setting endpoint: %v", err)
	}

	appName := c.Args().Get(0)
	wrapper, _, err := a.AppsAppGet(appName)
	if err != nil {
		return fmt.Errorf("error creating app: %v", err)
	}

	fmt.Println("app:", wrapper.App.Name)
	if config := wrapper.App.Config; len(config) > 0 {
		fmt.Println("configuration:")
		w := tabwriter.NewWriter(os.Stdout, 0, 8, 1, ' ', 0)
		fmt.Fprintln(w, "key\tvalue")
		for k, v := range wrapper.App.Config {
			fmt.Fprint(w, k, "\t", v, "\n")
		}
		w.Flush()
	}
	return nil
}
