package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
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
		ArgsUsage: "fn apps",
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
				Name:   "config",
				Usage:  "operate an application configuration set",
				Action: a.configList,
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "shell",
						Usage: "output in shell format",
					},
					cli.BoolFlag{
						Name:  "json",
						Usage: "output in JSON format",
					},
				},
				Subcommands: []cli.Command{
					{
						Name:        "set",
						Description: "store a configuration key for this application",
						Usage:       "<app> <key> <value>",
						Action:      a.configSet,
					},
					{
						Name:        "unset",
						Description: "remove a configuration key for this application",
						Usage:       "<app> <key> <value>",
						Action:      a.configUnset,
					},
				},
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

	body := functions.AppWrapper{App: functions.App{
		Name:   c.Args().Get(0),
		Config: extractEnvConfig(c.StringSlice("config")),
	}}
	wrapper, _, err := a.AppsPost(body)
	if err != nil {
		return fmt.Errorf("error creating app: %v", err)
	}

	fmt.Println(wrapper.App.Name, "created")
	return nil
}

func (a *appsCmd) configList(c *cli.Context) error {
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

	config := wrapper.App.Config
	if len(config) == 0 {
		return errors.New("this application has no configurations")
	}

	if c.Bool("json") {
		if err := json.NewEncoder(os.Stdout).Encode(config); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else if c.Bool("shell") {
		for k, v := range wrapper.App.Config {
			fmt.Print("export ", k, "=", v, "\n")
		}
	} else {
		fmt.Println(wrapper.App.Name, "configuration:")
		w := tabwriter.NewWriter(os.Stdout, 0, 8, 1, ' ', 0)
		for k, v := range config {
			fmt.Fprint(w, k, ":\t", v, "\n")
		}
		w.Flush()
	}
	return nil
}

func (a *appsCmd) configSet(c *cli.Context) error {
	if c.Args().Get(0) == "" || c.Args().Get(1) == "" || c.Args().Get(2) == "" {
		return errors.New("error: application configuration setting takes three arguments: an app name, a key and a value")
	}

	if err := resetBasePath(&a.Configuration); err != nil {
		return fmt.Errorf("error setting endpoint: %v", err)
	}

	appName := c.Args().Get(0)
	key := c.Args().Get(1)
	value := c.Args().Get(2)

	wrapper, _, err := a.AppsAppGet(appName)
	if err != nil {
		return fmt.Errorf("error creating app: %v", err)
	}

	config := wrapper.App.Config

	if config == nil {
		config = make(map[string]string)
	}

	config[key] = value

	if err := a.storeApp(appName, config); err != nil {
		return fmt.Errorf("error updating app configuration: %v", err)
	}

	fmt.Println(wrapper.App.Name, "updated", key, "with", value)
	return nil
}

func (a *appsCmd) configUnset(c *cli.Context) error {
	if c.Args().Get(0) == "" || c.Args().Get(1) == "" {
		return errors.New("error: application configuration setting takes three arguments: an app name, a key and a value")
	}

	if err := resetBasePath(&a.Configuration); err != nil {
		return fmt.Errorf("error setting endpoint: %v", err)
	}

	appName := c.Args().Get(0)
	key := c.Args().Get(1)

	wrapper, _, err := a.AppsAppGet(appName)
	if err != nil {
		return fmt.Errorf("error creating app: %v", err)
	}

	config := wrapper.App.Config

	if config == nil {
		config = make(map[string]string)
	}

	if _, ok := config[key]; !ok {
		return fmt.Errorf("configuration key %s not found", key)
	}

	delete(config, key)

	if err := a.storeApp(appName, config); err != nil {
		return fmt.Errorf("error updating app configuration: %v", err)
	}

	fmt.Println(wrapper.App.Name, "removed", key)
	return nil
}

func (a *appsCmd) storeApp(appName string, config map[string]string) error {
	body := functions.AppWrapper{App: functions.App{
		Name:   appName,
		Config: config,
	}}

	if _, _, err := a.AppsPost(body); err != nil {
		return fmt.Errorf("error updating app configuration: %v", err)
	}
	return nil
}
