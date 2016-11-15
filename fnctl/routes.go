package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"text/tabwriter"

	functions "github.com/iron-io/functions_go"
	"github.com/urfave/cli"
	"golang.org/x/crypto/ssh/terminal"
)

type routesCmd struct {
	*functions.RoutesApi
}

func routes() cli.Command {
	r := routesCmd{RoutesApi: functions.NewRoutesApi()}

	return cli.Command{
		Name:      "routes",
		Usage:     "list routes",
		ArgsUsage: "fnctl routes",
		Action:    r.list,
		Subcommands: []cli.Command{
			{
				Name:      "call",
				Usage:     "call a route",
				ArgsUsage: "appName /path",
				Action:    r.call,
				Flags:     runflags(),
			},
			{
				Name:      "create",
				Usage:     "create a route",
				ArgsUsage: "appName /path image/name",
				Action:    r.create,
				Flags: []cli.Flag{
					cli.Int64Flag{
						Name:  "memory",
						Usage: "memory in MiB",
						Value: 128,
					},
					cli.StringFlag{
						Name:  "type",
						Usage: "route type - sync or async",
						Value: "sync",
					},
					cli.StringSliceFlag{
						Name:  "config",
						Usage: "route configuration",
					},
				},
			},
			{
				Name:      "delete",
				Usage:     "delete a route",
				ArgsUsage: "appName /path",
				Action:    r.delete,
			},
		},
	}
}

func call() cli.Command {
	r := routesCmd{RoutesApi: functions.NewRoutesApi()}

	return cli.Command{
		Name:      "call",
		Usage:     "call a remote function",
		ArgsUsage: "appName /path",
		Flags:     runflags(),
		Action:    r.call,
	}
}

func (a *routesCmd) list(c *cli.Context) error {
	if c.Args().First() == "" {
		return errors.New("error: routes listing takes one argument, an app name")
	}

	if err := resetBasePath(&a.Configuration); err != nil {
		return fmt.Errorf("error setting endpoint: %v", err)
	}

	appName := c.Args().Get(0)
	wrapper, _, err := a.AppsAppRoutesGet(appName)
	if err != nil {
		return fmt.Errorf("error getting routes: %v", err)
	}

	baseURL, err := url.Parse(a.Configuration.BasePath)
	if err != nil {
		return fmt.Errorf("error parsing base path: %v", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 0, '\t', 0)
	fmt.Fprint(w, "path", "\t", "image", "\t", "endpoint", "\n")
	for _, route := range wrapper.Routes {
		u, err := url.Parse("../")
		u.Path = path.Join(u.Path, "r", appName, route.Path)
		if err != nil {
			return fmt.Errorf("error parsing functions route path: %v", err)
		}

		fmt.Fprint(w, route.Path, "\t", route.Image, "\t", baseURL.ResolveReference(u).String(), "\n")
	}
	w.Flush()

	return nil
}

func (a *routesCmd) call(c *cli.Context) error {
	if c.Args().Get(0) == "" || c.Args().Get(1) == "" {
		return errors.New("error: routes listing takes three arguments: an app name and a route")
	}

	if err := resetBasePath(&a.Configuration); err != nil {
		return fmt.Errorf("error setting endpoint: %v", err)
	}

	baseURL, err := url.Parse(a.Configuration.BasePath)
	if err != nil {
		return fmt.Errorf("error parsing base path: %v", err)
	}

	appName := c.Args().Get(0)
	route := c.Args().Get(1)

	u, err := url.Parse("../")
	u.Path = path.Join(u.Path, "r", appName, route)

	var content io.Reader
	if !terminal.IsTerminal(int(os.Stdin.Fd())) {
		content = os.Stdin
	}

	req, err := http.NewRequest("POST", baseURL.ResolveReference(u).String(), content)
	if err != nil {
		return fmt.Errorf("error running route: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	envAsHeader(req, c.StringSlice("e"))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error running route: %v", err)
	}

	io.Copy(os.Stdout, resp.Body)
	return nil
}

func envAsHeader(req *http.Request, selectedEnv []string) {
	detectedEnv := os.Environ()
	if len(selectedEnv) > 0 {
		detectedEnv = selectedEnv
	}

	for _, e := range detectedEnv {
		kv := strings.Split(e, "=")
		name := kv[0]
		req.Header.Set(name, os.Getenv(name))
	}
}

func (a *routesCmd) create(c *cli.Context) error {
	if c.Args().Get(0) == "" || c.Args().Get(1) == "" {
		return errors.New("error: routes creation takes three arguments: an app name, a route path and an image")
	}

	if err := resetBasePath(&a.Configuration); err != nil {
		return fmt.Errorf("error setting endpoint: %v", err)
	}

	appName := c.Args().Get(0)
	route := c.Args().Get(1)
	image := c.Args().Get(2)
	if image == "" {
		ff, err := findFuncfile()
		if err != nil {
			if _, ok := err.(*notFoundError); ok {
				return errors.New("error: image name is missing or no function file found")
			} else {
				return err
			}
		}
		image = ff.FullName()
	}

	body := functions.RouteWrapper{
		Route: functions.Route{
			AppName: appName,
			Path:    route,
			Image:   image,
			Memory:  c.Int64("memory"),
			Type_:   c.String("type"),
			Config:  extractEnvConfig(c.StringSlice("config")),
		},
	}

	wrapper, _, err := a.AppsAppRoutesPost(appName, body)
	if err != nil {
		return fmt.Errorf("error creating route: %v", err)
	}
	if wrapper.Route.Path == "" || wrapper.Route.Image == "" {
		return fmt.Errorf("could not create this route (%s at %s), check if route path is correct", route, appName)
	}

	fmt.Println(wrapper.Route.Path, "created with", wrapper.Route.Image)
	return nil
}

func (a *routesCmd) delete(c *cli.Context) error {
	if c.Args().Get(0) == "" || c.Args().Get(1) == "" {
		return errors.New("error: routes listing takes three arguments: an app name and a path")
	}

	if err := resetBasePath(&a.Configuration); err != nil {
		return fmt.Errorf("error setting endpoint: %v", err)
	}

	appName := c.Args().Get(0)
	route := c.Args().Get(1)
	_, err := a.AppsAppRoutesRouteDelete(appName, route)
	if err != nil {
		return fmt.Errorf("error deleting route: %v", err)
	}

	fmt.Println(route, "deleted")
	return nil
}
