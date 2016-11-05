// Copyright 2016 Iron.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
