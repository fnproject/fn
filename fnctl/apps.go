package main

import (
	"errors"
	"fmt"

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
		ArgsUsage: "fnclt apps",
		Flags:     append(confFlags(&a.Configuration), []cli.Flag{}...),
		Action:    a.list,
		Subcommands: []cli.Command{
			{
				Name:   "create",
				Usage:  "create a new app",
				Action: a.create,
			},
		},
	}
}

func (a *appsCmd) list(c *cli.Context) error {
	resetBasePath(&a.Configuration)

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

	resetBasePath(&a.Configuration)

	appName := c.Args().Get(0)
	body := functions.AppWrapper{App: functions.App{Name: appName}}
	wrapper, _, err := a.AppsPost(body)
	if err != nil {
		return fmt.Errorf("error creating app: %v", err)
	}

	fmt.Println(wrapper.App.Name, "created")
	return nil
}
