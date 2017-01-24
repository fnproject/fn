package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"context"
	fnclient "github.com/iron-io/functions_go/client"
	apiapps "github.com/iron-io/functions_go/client/apps"
	"github.com/iron-io/functions_go/models"
	"github.com/urfave/cli"
)

type appsCmd struct {
	client *fnclient.Functions
}

func apps() cli.Command {
	a := appsCmd{client: apiClient()}

	return cli.Command{
		Name:      "apps",
		Usage:     "manage applications",
		ArgsUsage: "fn apps",
		Subcommands: []cli.Command{
			{
				Name:      "create",
				Aliases:   []string{"c"},
				Usage:     "create a new app",
				ArgsUsage: "`app`",
				Action:    a.create,
				Flags: []cli.Flag{
					cli.StringSliceFlag{
						Name:  "config",
						Usage: "application configuration",
					},
				},
			},
			{
				Name:    "list",
				Aliases: []string{"l"},
				Usage:   "list all apps",
				Action:  a.list,
			},
			{
				Name:  "config",
				Usage: "operate an application configuration set",
				Subcommands: []cli.Command{
					{
						Name:      "view",
						Aliases:   []string{"v"},
						Usage:     "view all configuration keys for this app",
						ArgsUsage: "`app`",
						Action:    a.configList,
						Flags: []cli.Flag{
							cli.BoolFlag{
								Name:  "shell,s",
								Usage: "output in shell format",
							},
							cli.BoolFlag{
								Name:  "json,j",
								Usage: "output in JSON format",
							},
						},
					},
					{
						Name:      "set",
						Aliases:   []string{"s"},
						Usage:     "store a configuration key for this application",
						ArgsUsage: "`app` <key> <value>",
						Action:    a.configSet,
					},
					{
						Name:      "unset",
						Aliases:   []string{"u"},
						Usage:     "remove a configuration key for this application",
						ArgsUsage: "`app` <key>",
						Action:    a.configUnset,
					},
				},
			},
			{
				Name:   "delete",
				Usage:  "delete an app",
				Action: a.delete,
			},
		},
	}
}

func (a *appsCmd) list(c *cli.Context) error {
	resp, err := a.client.Apps.GetApps(&apiapps.GetAppsParams{
		Context: context.Background(),
	})

	if err != nil {
		switch err.(type) {
		case *apiapps.GetAppsAppNotFound:
			return fmt.Errorf("error: %v", err.(*apiapps.GetAppsAppNotFound).Payload.Error.Message)
		case *apiapps.GetAppsAppDefault:
			return fmt.Errorf("unexpected error: %v", err.(*apiapps.GetAppsAppDefault).Payload.Error.Message)
		}
		return fmt.Errorf("unexpected error: %v", err)
	}

	if len(resp.Payload.Apps) == 0 {
		fmt.Println("no apps found")
		return nil
	}

	for _, app := range resp.Payload.Apps {
		fmt.Println(app.Name)
	}

	return nil
}

func (a *appsCmd) create(c *cli.Context) error {
	if c.Args().First() == "" {
		return errors.New("error: app creating takes one argument, an app name")
	}

	body := &models.AppWrapper{App: &models.App{
		Name:   c.Args().Get(0),
		Config: extractEnvConfig(c.StringSlice("config")),
	}}

	resp, err := a.client.Apps.PostApps(&apiapps.PostAppsParams{
		Context: context.Background(),
		Body:    body,
	})

	if err != nil {
		switch err.(type) {
		case *apiapps.PostAppsBadRequest:
			return fmt.Errorf("error: %v", err.(*apiapps.PostAppsBadRequest).Payload.Error.Message)
		case *apiapps.PostAppsConflict:
			return fmt.Errorf("error: %v", err.(*apiapps.PostAppsConflict).Payload.Error.Message)
		case *apiapps.PostAppsDefault:
			return fmt.Errorf("unexpected error: %v", err.(*apiapps.PostAppsDefault).Payload.Error.Message)
		}
		return fmt.Errorf("unexpected error: %v", err)
	}

	fmt.Println(resp.Payload.App.Name, "created")
	return nil
}

func (a *appsCmd) configList(c *cli.Context) error {
	if c.Args().First() == "" {
		return errors.New("error: app description takes one argument, an app name")
	}

	appName := c.Args().Get(0)

	resp, err := a.client.Apps.GetAppsApp(&apiapps.GetAppsAppParams{
		Context: context.Background(),
		App:     appName,
	})

	if err != nil {
		switch err.(type) {
		case *apiapps.GetAppsAppNotFound:
			return fmt.Errorf("error: %v", err.(*apiapps.GetAppsAppNotFound).Payload.Error.Message)
		case *apiapps.GetAppsAppDefault:
			return fmt.Errorf("unexpected error: %v", err.(*apiapps.GetAppsAppDefault).Payload.Error.Message)
		}
		return fmt.Errorf("unexpected error: %v", err)
	}

	config := resp.Payload.App.Config
	if len(config) == 0 {
		return errors.New("this application has no configurations")
	}

	if c.Bool("json") {
		if err := json.NewEncoder(os.Stdout).Encode(config); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else if c.Bool("shell") {
		for k, v := range resp.Payload.App.Config {
			fmt.Print("export ", k, "=", v, "\n")
		}
	} else {
		fmt.Println(resp.Payload.App.Name, "configuration:")
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

	appName := c.Args().Get(0)
	key := c.Args().Get(1)
	value := c.Args().Get(2)

	resp, err := a.client.Apps.GetAppsApp(&apiapps.GetAppsAppParams{
		Context: context.Background(),
		App:     appName,
	})

	if err != nil {
		switch err.(type) {
		case *apiapps.GetAppsAppNotFound:
			return fmt.Errorf("error: %v", err.(*apiapps.GetAppsAppNotFound).Payload.Error.Message)
		case *apiapps.GetAppsAppDefault:
			return fmt.Errorf("unexpected error: %v", err.(*apiapps.GetAppsAppDefault).Payload.Error.Message)
		}
		return fmt.Errorf("unexpected error: %v", err)
	}

	config := resp.Payload.App.Config

	if config == nil {
		config = make(map[string]string)
	}

	config[key] = value

	if err := a.patchApp(appName, config); err != nil {
		return fmt.Errorf("error updating app configuration: %v", err)
	}

	fmt.Println(resp.Payload.App.Name, "updated", key, "with", value)
	return nil
}

func (a *appsCmd) configUnset(c *cli.Context) error {
	if c.Args().Get(0) == "" || c.Args().Get(1) == "" {
		return errors.New("error: application configuration setting takes three arguments: an app name, a key and a value")
	}

	appName := c.Args().Get(0)
	key := c.Args().Get(1)

	resp, err := a.client.Apps.GetAppsApp(&apiapps.GetAppsAppParams{
		Context: context.Background(),
		App:     appName,
	})

	if err != nil {
		switch err.(type) {
		case *apiapps.GetAppsAppNotFound:
			return fmt.Errorf("error: %v", err.(*apiapps.GetAppsAppNotFound).Payload.Error.Message)
		case *apiapps.GetAppsAppDefault:
			return fmt.Errorf("unexpected error: %v", err.(*apiapps.GetAppsAppDefault).Payload.Error.Message)
		}
		return fmt.Errorf("unexpected error: %v", err)
	}

	config := resp.Payload.App.Config

	if config == nil {
		config = make(map[string]string)
	}

	if _, ok := config[key]; !ok {
		return fmt.Errorf("configuration key %s not found", key)
	}

	delete(config, key)

	if err := a.patchApp(appName, config); err != nil {
		return fmt.Errorf("error updating app configuration: %v", err)
	}

	fmt.Println(resp.Payload.App.Name, "removed", key)
	return nil
}

func (a *appsCmd) patchApp(appName string, config map[string]string) error {
	body := &models.AppWrapper{App: &models.App{
		Config: config,
	}}

	_, err := a.client.Apps.PatchAppsApp(&apiapps.PatchAppsAppParams{
		Context: context.Background(),
		App:     appName,
		Body:    body,
	})

	if err != nil {
		switch err.(type) {
		case *apiapps.PatchAppsAppBadRequest:
			return errors.New(err.(*apiapps.PatchAppsAppBadRequest).Payload.Error.Message)
		case *apiapps.PatchAppsAppNotFound:
			return errors.New(err.(*apiapps.PatchAppsAppNotFound).Payload.Error.Message)
		case *apiapps.PatchAppsAppDefault:
			return errors.New(err.(*apiapps.PatchAppsAppDefault).Payload.Error.Message)
		}
		return fmt.Errorf("unexpected error: %v", err)
	}

	return nil
}

func (a *appsCmd) delete(c *cli.Context) error {
	appName := c.Args().First()
	if appName == "" {
		return errors.New("error: deleting an app takes one argument, an app name")
	}

	_, err := a.client.Apps.DeleteAppsApp(&apiapps.DeleteAppsAppParams{
		Context: context.Background(),
		App:     appName,
	})

	if err != nil {
		switch err.(type) {
		case *apiapps.DeleteAppsAppNotFound:
			return errors.New(err.(*apiapps.DeleteAppsAppNotFound).Payload.Error.Message)
		case *apiapps.DeleteAppsAppDefault:
			return errors.New(err.(*apiapps.DeleteAppsAppDefault).Payload.Error.Message)
		}
		return fmt.Errorf("unexpected error: %v", err)
	}

	fmt.Println(appName, "deleted")
	return nil
}
