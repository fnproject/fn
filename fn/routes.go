package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"text/tabwriter"
	"time"

	functions "github.com/iron-io/functions_go"
	"github.com/jmoiron/jsonq"
	"github.com/urfave/cli"
)

type routesCmd struct {
	*functions.RoutesApi
}

func routes() cli.Command {
	r := routesCmd{RoutesApi: functions.NewRoutesApi()}

	return cli.Command{
		Name:      "routes",
		Usage:     "manage routes",
		ArgsUsage: "fn routes",
		Subcommands: []cli.Command{
			{
				Name:      "call",
				Usage:     "call a route",
				ArgsUsage: "`app` /path",
				Action:    r.call,
				Flags:     runflags(),
			},
			{
				Name:      "list",
				Aliases:   []string{"l"},
				Usage:     "list routes for `app`",
				ArgsUsage: "`app`",
				Action:    r.list,
			},
			{
				Name:      "create",
				Aliases:   []string{"c"},
				Usage:     "create a route in an `app`",
				ArgsUsage: "`app` /path image/name",
				Action:    r.create,
				Flags: []cli.Flag{
					cli.Int64Flag{
						Name:  "memory,m",
						Usage: "memory in MiB",
						Value: 128,
					},
					cli.StringFlag{
						Name:  "type,t",
						Usage: "route type - sync or async",
						Value: "sync",
					},
					cli.StringSliceFlag{
						Name:  "config,c",
						Usage: "route configuration",
					},
					cli.StringFlag{
						Name:  "format,f",
						Usage: "hot function IO format - json or http",
						Value: "",
					},
					cli.IntFlag{
						Name:  "max-concurrency",
						Usage: "maximum concurrency for hot function",
						Value: 1,
					},
					cli.DurationFlag{
						Name:  "timeout",
						Usage: "route timeout",
						Value: 30 * time.Second,
					},
				},
			},
			{
				Name:      "update",
				Aliases:   []string{"u"},
				Usage:     "update a route in an `app`",
				ArgsUsage: "`app` /path",
				Action:    r.update,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "image,i",
						Usage: "image name",
					},
					cli.Int64Flag{
						Name:  "memory,m",
						Usage: "memory in MiB",
						Value: 128,
					},
					cli.StringFlag{
						Name:  "type,t",
						Usage: "route type - sync or async",
						Value: "sync",
					},
					cli.StringSliceFlag{
						Name:  "config,c",
						Usage: "route configuration",
					},
					cli.StringSliceFlag{
						Name:  "headers",
						Usage: "route response headers",
					},
					cli.StringFlag{
						Name:  "format,f",
						Usage: "hot container IO format - json or http",
						Value: "",
					},
					cli.IntFlag{
						Name:  "max-concurrency",
						Usage: "maximum concurrency for hot container",
						Value: 1,
					},
					cli.DurationFlag{
						Name:  "timeout",
						Usage: "route timeout",
						Value: 30 * time.Second,
					},
				},
			},
			{
				Name:      "delete",
				Aliases:   []string{"d"},
				Usage:     "delete a route from `app`",
				ArgsUsage: "`app` /path",
				Action:    r.delete,
			},
			{
				Name:      "inspect",
				Aliases:   []string{"i"},
				Usage:     "retrieve one or all routes properties",
				ArgsUsage: "`app` /path [property.[key]]",
				Action:    r.inspect,
			},
		},
	}
}

func call() cli.Command {
	r := routesCmd{RoutesApi: functions.NewRoutesApi()}

	return cli.Command{
		Name:      "call",
		Usage:     "call a remote function",
		ArgsUsage: "`app` /path",
		Flags:     runflags(),
		Action:    r.call,
	}
}

func (a *routesCmd) list(c *cli.Context) error {
	if c.Args().First() == "" {
		return errors.New("error: routes listing takes one argument, an app name")
	}

	if err := resetBasePath(a.Configuration); err != nil {
		return fmt.Errorf("error setting endpoint: %v", err)
	}

	appName := c.Args().Get(0)
	wrapper, _, err := a.AppsAppRoutesGet(appName)
	if err != nil {
		return fmt.Errorf("error getting routes: %v", err)
	}

	if msg := wrapper.Error_.Message; msg != "" {
		return errors.New(msg)
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

	if err := resetBasePath(a.Configuration); err != nil {
		return fmt.Errorf("error setting endpoint: %v", err)
	}

	appName := c.Args().Get(0)
	route := c.Args().Get(1)

	baseURL, err := url.Parse(a.Configuration.BasePath)
	if err != nil {
		return fmt.Errorf("error parsing base path: %v", err)
	}

	u, err := url.Parse("../")
	u.Path = path.Join(u.Path, "r", appName, route)
	content := stdin()

	return callfn(baseURL.ResolveReference(u).String(), content, os.Stdout, c.StringSlice("e"))
}

func callfn(u string, content io.Reader, output io.Writer, env []string) error {
	req, err := http.NewRequest("POST", u, content)
	if err != nil {
		return fmt.Errorf("error running route: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if len(env) > 0 {
		envAsHeader(req, env)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error running route: %v", err)
	}

	io.Copy(output, resp.Body)
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
	if c.Args().Get(0) == "" {
		return errors.New("error: routes creation takes at least one argument: an app name")
	}

	if err := resetBasePath(a.Configuration); err != nil {
		return fmt.Errorf("error setting endpoint: %v", err)
	}

	appName := c.Args().Get(0)
	route := c.Args().Get(1)
	image := c.Args().Get(2)
	var (
		format  string
		maxC    int
		timeout time.Duration
	)
	if image == "" {
		ff, err := loadFuncfile()
		if err != nil {
			if _, ok := err.(*notFoundError); ok {
				return errors.New("error: image name is missing or no function file found")
			} else {
				return err
			}
		}
		image = ff.FullName()
		if ff.Format != nil {
			format = *ff.Format
		}
		if ff.MaxConcurrency != nil {
			maxC = *ff.MaxConcurrency
		}
		if ff.Timeout != nil {
			timeout = *ff.Timeout
		}
		if route == "" && ff.Path != nil {
			route = *ff.Path
		}
	}

	if route == "" {
		return errors.New("error: route path is missing")
	}
	if image == "" {
		return errors.New("error: function image name is missing")
	}

	if f := c.String("format"); f != "" {
		format = f
	}
	if m := c.Int("max-concurrency"); m > 0 {
		maxC = m
	}
	if t := c.Duration("timeout"); t > 0 {
		timeout = t
	}

	body := functions.RouteWrapper{
		Route: functions.Route{
			Path:           route,
			Image:          image,
			Memory:         c.Int64("memory"),
			Type_:          c.String("type"),
			Config:         extractEnvConfig(c.StringSlice("config")),
			Format:         format,
			MaxConcurrency: int32(maxC),
			Timeout:        int32(timeout.Seconds()),
		},
	}

	wrapper, _, err := a.AppsAppRoutesPost(appName, body)
	if err != nil {
		return fmt.Errorf("error creating route: %v", err)
	}

	if msg := wrapper.Error_.Message; msg != "" {
		return errors.New(msg)
	}

	fmt.Println(wrapper.Route.Path, "created with", wrapper.Route.Image)
	return nil
}

func (a *routesCmd) delete(c *cli.Context) error {
	if c.Args().Get(0) == "" || c.Args().Get(1) == "" {
		return errors.New("error: routes listing takes three arguments: an app name and a path")
	}

	if err := resetBasePath(a.Configuration); err != nil {
		return fmt.Errorf("error setting endpoint: %v", err)
	}

	appName := c.Args().Get(0)
	route := c.Args().Get(1)

	resp, err := a.AppsAppRoutesRouteDelete(appName, route)
	if err != nil {
		return fmt.Errorf("error deleting route: %v", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("route not found: %s", route)
	}

	fmt.Println(route, "deleted")
	return nil
}

func (a *routesCmd) update(c *cli.Context) error {
	if c.Args().Get(0) == "" {
		return errors.New("error: routes creation takes at least one argument: an app name")
	}

	if err := resetBasePath(a.Configuration); err != nil {
		return fmt.Errorf("error setting endpoint: %v", err)
	}

	appName := c.Args().Get(0)
	route := c.Args().Get(1)

	if route == "" {
		return errors.New("error: route path is missing")
	}

	headers := map[string][]string{}
	for _, header := range c.StringSlice("headers") {
		parts := strings.Split(header, "=")
		headers[parts[0]] = strings.Split(parts[1], ";")
	}

	patchedRoute := &functions.Route{
		Path:           route,
		Image:          c.String("image"),
		Memory:         c.Int64("memory"),
		Type_:          c.String("type"),
		Config:         extractEnvConfig(c.StringSlice("config")),
		Headers:        headers,
		Format:         c.String("format"),
		MaxConcurrency: int32(c.Int64("max-concurrency")),
		Timeout:        int32(c.Int64("timeout")),
	}

	err := a.patchRoute(appName, route, patchedRoute)
	if err != nil {
		return err
	}

	fmt.Println(appName, route, "updated")
	return nil
}

func (a *routesCmd) configList(c *cli.Context) error {
	if c.Args().Get(0) == "" || c.Args().Get(1) == "" {
		return errors.New("error: route configuration description takes two arguments: an app name and a route")
	}

	if err := resetBasePath(a.Configuration); err != nil {
		return fmt.Errorf("error setting endpoint: %v", err)
	}

	appName := c.Args().Get(0)
	route := c.Args().Get(1)
	wrapper, _, err := a.AppsAppRoutesRouteGet(appName, route)
	if err != nil {
		return fmt.Errorf("error loading route information: %v", err)
	}

	if msg := wrapper.Error_.Message; msg != "" {
		return errors.New(msg)
	}

	config := wrapper.Route.Config
	if len(config) == 0 {
		return errors.New("this route has no configurations")
	}

	if c.Bool("json") {
		if err := json.NewEncoder(os.Stdout).Encode(config); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else if c.Bool("shell") {
		for k, v := range config {
			fmt.Print("export ", k, "=", v, "\n")
		}
	} else {
		fmt.Println(appName, wrapper.Route.Path, "configuration:")
		w := tabwriter.NewWriter(os.Stdout, 0, 8, 1, ' ', 0)
		for k, v := range config {
			fmt.Fprint(w, k, ":\t", v, "\n")
		}
		w.Flush()
	}
	return nil
}

func (a *routesCmd) patchRoute(appName, routePath string, r *functions.Route) error {
	wrapper, _, err := a.AppsAppRoutesRouteGet(appName, routePath)
	if err != nil {
		return fmt.Errorf("error loading route: %v", err)
	}

	if msg := wrapper.Error_.Message; msg != "" {
		return errors.New(msg)
	}

	wrapper.Route.Path = ""
	if r != nil {
		if r.Config != nil {
			for k, v := range r.Config {
				if v == "" {
					delete(r.Config, k)
					continue
				}
				wrapper.Route.Config[k] = v
			}
		}
		if r.Headers != nil {
			for k, v := range r.Headers {
				if v[0] == "" {
					delete(r.Headers, k)
					continue
				}
				wrapper.Route.Headers[k] = v
			}
		}
		if r.Image != "" {
			wrapper.Route.Image = r.Image
		}
		if r.Format != "" {
			wrapper.Route.Format = r.Format
		}
		if r.MaxConcurrency > 0 {
			wrapper.Route.MaxConcurrency = r.MaxConcurrency
		}
		if r.Memory > 0 {
			wrapper.Route.Memory = r.Memory
		}
		if r.Timeout > 0 {
			wrapper.Route.Timeout = r.Timeout
		}
	}

	if wrapper, _, err = a.AppsAppRoutesRoutePatch(appName, routePath, *wrapper); err != nil {
		return fmt.Errorf("error updating route: %v", err)
	}

	if msg := wrapper.Error_.Message; msg != "" {
		return errors.New(msg)
	}

	return nil
}

func (a *routesCmd) inspect(c *cli.Context) error {
	if c.Args().Get(0) == "" || c.Args().Get(1) == "" {
		return errors.New("error: routes listing takes three arguments: an app name and a path")
	}

	if err := resetBasePath(a.Configuration); err != nil {
		return fmt.Errorf("error setting endpoint: %v", err)
	}

	appName := c.Args().Get(0)
	route := c.Args().Get(1)
	prop := c.Args().Get(2)

	wrapper, resp, err := a.AppsAppRoutesRouteGet(appName, route)
	if err != nil {
		return fmt.Errorf("error retrieving route: %v", err)
	}

	if msg := wrapper.Error_.Message; msg != "" {
		return errors.New(msg)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "\t")

	if prop == "" {
		enc.Encode(wrapper.Route)
		return nil
	}

	var inspect struct{ Route map[string]interface{} }
	err = json.Unmarshal(resp.Payload, &inspect)
	if err != nil {
		return fmt.Errorf("error inspect route: %v", err)
	}

	jq := jsonq.NewQuery(inspect.Route)
	field, err := jq.Interface(strings.Split(prop, ".")...)
	if err != nil {
		return errors.New("failed to inspect the property")
	}
	enc.Encode(field)

	return nil
}
