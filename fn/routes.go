package main

import (
	"context"
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

	fnclient "github.com/iron-io/functions_go/client"
	apiroutes "github.com/iron-io/functions_go/client/routes"
	"github.com/iron-io/functions_go/models"
	fnmodels "github.com/iron-io/functions_go/models"
	"github.com/jmoiron/jsonq"
	"github.com/urfave/cli"
)

type routesCmd struct {
	client *fnclient.Functions
}

var routeFlags = []cli.Flag{
	cli.StringFlag{
		Name:  "image,i",
		Usage: "image name",
	},
	cli.Int64Flag{
		Name:  "memory,m",
		Usage: "memory in MiB",
	},
	cli.StringFlag{
		Name:  "type,t",
		Usage: "route type - sync or async",
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
	},
	cli.IntFlag{
		Name:  "max-concurrency,mc",
		Usage: "maximum concurrency for hot container",
	},
	cli.DurationFlag{
		Name:  "timeout",
		Usage: "route timeout (eg. 30s)",
	},
}

func routes() cli.Command {

	r := routesCmd{client: apiClient()}

	return cli.Command{
		Name:  "routes",
		Usage: "manage routes",
		Subcommands: []cli.Command{
			{
				Name:      "call",
				Usage:     "call a route",
				ArgsUsage: "<app> </path> [image]",
				Action:    r.call,
				Flags:     runflags(),
			},
			{
				Name:      "list",
				Aliases:   []string{"l"},
				Usage:     "list routes for `app`",
				ArgsUsage: "<app>",
				Action:    r.list,
			},
			{
				Name:      "create",
				Aliases:   []string{"c"},
				Usage:     "create a route in an `app`",
				ArgsUsage: "<app> </path>",
				Action:    r.create,
				Flags:     routeFlags,
			},
			{
				Name:      "update",
				Aliases:   []string{"u"},
				Usage:     "update a route in an `app`",
				ArgsUsage: "<app> </path>",
				Action:    r.update,
				Flags:     routeFlags,
			},
			{
				Name:  "config",
				Usage: "operate a route configuration set",
				Subcommands: []cli.Command{
					{
						Name:      "set",
						Aliases:   []string{"s"},
						Usage:     "store a configuration key for this route",
						ArgsUsage: "<app> </path> <key> <value>",
						Action:    r.configSet,
					},
					{
						Name:      "unset",
						Aliases:   []string{"u"},
						Usage:     "remove a configuration key for this route",
						ArgsUsage: "<app> </path> <key>",
						Action:    r.configUnset,
					},
				},
			},
			{
				Name:      "delete",
				Aliases:   []string{"d"},
				Usage:     "delete a route from `app`",
				ArgsUsage: "<app> </path>",
				Action:    r.delete,
			},
			{
				Name:      "inspect",
				Aliases:   []string{"i"},
				Usage:     "retrieve one or all routes properties",
				ArgsUsage: "<app> </path> [property.[key]]",
				Action:    r.inspect,
			},
		},
	}
}

func call() cli.Command {
	r := routesCmd{client: apiClient()}

	return cli.Command{
		Name:      "call",
		Usage:     "call a remote function",
		ArgsUsage: "<app> </path>",
		Flags:     runflags(),
		Action:    r.call,
	}
}

func cleanRoutePath(p string) string {
	p = path.Clean(p)
	if !path.IsAbs(p) {
		p = "/" + p
	}
	return p
}

func (a *routesCmd) list(c *cli.Context) error {
	appName := c.Args().Get(0)

	resp, err := a.client.Routes.GetAppsAppRoutes(&apiroutes.GetAppsAppRoutesParams{
		Context: context.Background(),
		App:     appName,
	})

	if err != nil {
		switch err.(type) {
		case *apiroutes.GetAppsAppRoutesNotFound:
			return fmt.Errorf("error: %s", err.(*apiroutes.GetAppsAppRoutesNotFound).Payload.Error.Message)
		case *apiroutes.GetAppsAppRoutesDefault:
			return fmt.Errorf("unexpected error: %s", err.(*apiroutes.GetAppsAppRoutesDefault).Payload.Error.Message)
		}
		return fmt.Errorf("unexpected error: %s", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 0, '\t', 0)
	fmt.Fprint(w, "path", "\t", "image", "\t", "endpoint", "\n")
	for _, route := range resp.Payload.Routes {
		u, err := url.Parse("../")
		u.Path = path.Join(u.Path, "r", appName, route.Path)
		if err != nil {
			return fmt.Errorf("error parsing functions route path: %s", err)
		}

		fmt.Fprint(w, route.Path, "\t", route.Image, "\n")
	}
	w.Flush()

	return nil
}

func (a *routesCmd) call(c *cli.Context) error {
	appName := c.Args().Get(0)
	route := cleanRoutePath(c.Args().Get(1))

	u := url.URL{
		Scheme: "http",
		Host:   host(),
	}
	u.Path = path.Join(u.Path, "r", appName, route)
	content := stdin()

	return callfn(u.String(), content, os.Stdout, c.String("method"), c.StringSlice("e"))
}

func callfn(u string, content io.Reader, output io.Writer, method string, env []string) error {
	if method == "" {
		if content == nil {
			method = "GET"
		} else {
			method = "POST"
		}
	}

	req, err := http.NewRequest(method, u, content)
	if err != nil {
		return fmt.Errorf("error running route: %s", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if len(env) > 0 {
		envAsHeader(req, env)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error running route: %s", err)
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

func routeWithFlags(c *cli.Context, rt *models.Route) {
	if i := c.String("image"); i != "" {
		rt.Image = i
	}

	if f := c.String("format"); f != "" {
		rt.Format = f
	}

	if t := c.String("type"); t != "" {
		rt.Type = t
	}

	if m := c.Int("max-concurrency"); m > 0 {
		rt.MaxConcurrency = int32(m)
	}

	if m := c.Int64("memory"); m > 0 {
		rt.Memory = m
	}

	if t := c.Duration("timeout"); t > 0 {
		to := int64(t.Seconds())
		rt.Timeout = &to
	}

	if len(c.StringSlice("headers")) > 0 {
		headers := map[string][]string{}
		for _, header := range c.StringSlice("headers") {
			parts := strings.Split(header, "=")
			headers[parts[0]] = strings.Split(parts[1], ";")
		}
		rt.Headers = headers
	}

	if len(c.StringSlice("config")) > 0 {
		rt.Config = extractEnvConfig(c.StringSlice("config"))
	}
}

func routeWithFuncFile(c *cli.Context, rt *models.Route) {
	ff, err := loadFuncfile()
	if err == nil {
		if ff.FullName() != "" { // args take precedence
			rt.Image = ff.FullName()
		}
		if ff.Format != nil {
			rt.Format = *ff.Format
		}
		if ff.MaxConcurrency != nil {
			rt.MaxConcurrency = int32(*ff.MaxConcurrency)
		}
		if ff.Timeout != nil {
			to := int64(ff.Timeout.Seconds())
			rt.Timeout = &to
		}
		if rt.Path == "" && ff.Path != nil {
			rt.Path = *ff.Path
		}
	}
}

func (a *routesCmd) create(c *cli.Context) error {
	appName := c.Args().Get(0)
	route := cleanRoutePath(c.Args().Get(1))

	rt := &models.Route{}
	rt.Path = route
	rt.Image = c.Args().Get(2)

	routeWithFuncFile(c, rt)
	routeWithFlags(c, rt)

	if rt.Path == "" {
		return errors.New("error: route path is missing")
	}
	if rt.Image == "" {
		fmt.Println("No image specified, using `iron/hello`")
		rt.Image = "iron/hello"
	}

	body := &models.RouteWrapper{
		Route: rt,
	}

	resp, err := a.client.Routes.PostAppsAppRoutes(&apiroutes.PostAppsAppRoutesParams{
		Context: context.Background(),
		App:     appName,
		Body:    body,
	})

	if err != nil {
		switch err.(type) {
		case *apiroutes.PostAppsAppRoutesBadRequest:
			return fmt.Errorf("error: %s", err.(*apiroutes.PostAppsAppRoutesBadRequest).Payload.Error.Message)
		case *apiroutes.PostAppsAppRoutesConflict:
			return fmt.Errorf("error: %s", err.(*apiroutes.PostAppsAppRoutesConflict).Payload.Error.Message)
		case *apiroutes.PostAppsAppRoutesDefault:
			return fmt.Errorf("unexpected error: %s", err.(*apiroutes.PostAppsAppRoutesDefault).Payload.Error.Message)
		}
		return fmt.Errorf("unexpected error: %s", err)
	}

	fmt.Println(resp.Payload.Route.Path, "created with", resp.Payload.Route.Image)
	return nil
}

func (a *routesCmd) patchRoute(appName, routePath string, r *fnmodels.Route) error {
	resp, err := a.client.Routes.GetAppsAppRoutesRoute(&apiroutes.GetAppsAppRoutesRouteParams{
		Context: context.Background(),
		App:     appName,
		Route:   routePath,
	})

	if err != nil {
		switch err.(type) {
		case *apiroutes.GetAppsAppRoutesRouteNotFound:
			return fmt.Errorf("error: %s", err.(*apiroutes.GetAppsAppRoutesRouteNotFound).Payload.Error.Message)
		case *apiroutes.GetAppsAppRoutesDefault:
			return fmt.Errorf("unexpected error: %s", err.(*apiroutes.GetAppsAppRoutesDefault).Payload.Error.Message)
		}
		return fmt.Errorf("unexpected error: %s", err)
	}

	if resp.Payload.Route.Config == nil {
		resp.Payload.Route.Config = map[string]string{}
	}

	if resp.Payload.Route.Headers == nil {
		resp.Payload.Route.Headers = map[string][]string{}
	}

	resp.Payload.Route.Path = ""
	if r != nil {
		if r.Config != nil {
			for k, v := range r.Config {
				if string(k[0]) == "-" {
					delete(resp.Payload.Route.Config, string(k[1:]))
					continue
				}
				resp.Payload.Route.Config[k] = v
			}
		}
		if r.Headers != nil {
			for k, v := range r.Headers {
				if string(k[0]) == "-" {
					delete(resp.Payload.Route.Headers, k)
					continue
				}
				resp.Payload.Route.Headers[k] = v
			}
		}
		if r.Image != "" {
			resp.Payload.Route.Image = r.Image
		}
		if r.Format != "" {
			resp.Payload.Route.Format = r.Format
		}
		if r.Type != "" {
			resp.Payload.Route.Type = r.Type
		}
		if r.MaxConcurrency > 0 {
			resp.Payload.Route.MaxConcurrency = r.MaxConcurrency
		}
		if r.Memory > 0 {
			resp.Payload.Route.Memory = r.Memory
		}
		if r.Timeout != nil {
			resp.Payload.Route.Timeout = r.Timeout
		}
	}

	_, err = a.client.Routes.PatchAppsAppRoutesRoute(&apiroutes.PatchAppsAppRoutesRouteParams{
		Context: context.Background(),
		App:     appName,
		Route:   routePath,
		Body:    resp.Payload,
	})

	if err != nil {
		switch err.(type) {
		case *apiroutes.PatchAppsAppRoutesRouteBadRequest:
			return fmt.Errorf("error: %s", err.(*apiroutes.PatchAppsAppRoutesRouteBadRequest).Payload.Error.Message)
		case *apiroutes.PatchAppsAppRoutesRouteNotFound:
			return fmt.Errorf("error: %s", err.(*apiroutes.PatchAppsAppRoutesRouteNotFound).Payload.Error.Message)
		case *apiroutes.PatchAppsAppRoutesRouteDefault:
			return fmt.Errorf("unexpected error: %s", err.(*apiroutes.PatchAppsAppRoutesRouteDefault).Payload.Error.Message)
		}
		return fmt.Errorf("unexpected error: %s", err)
	}

	return nil
}

func (a *routesCmd) update(c *cli.Context) error {
	appName := c.Args().Get(0)
	route := cleanRoutePath(c.Args().Get(1))

	rt := &models.Route{}
	routeWithFuncFile(c, rt)
	routeWithFlags(c, rt)

	err := a.patchRoute(appName, route, rt)
	if err != nil {
		return err
	}

	fmt.Println(appName, route, "updated")
	return nil
}

func (a *routesCmd) configSet(c *cli.Context) error {
	appName := c.Args().Get(0)
	route := cleanRoutePath(c.Args().Get(1))
	key := c.Args().Get(2)
	value := c.Args().Get(3)

	patchRoute := fnmodels.Route{
		Config: make(map[string]string),
	}

	patchRoute.Config[key] = value

	err := a.patchRoute(appName, route, &patchRoute)
	if err != nil {
		return err
	}

	fmt.Println(appName, route, "updated", key, "with", value)
	return nil
}

func (a *routesCmd) configUnset(c *cli.Context) error {
	appName := c.Args().Get(0)
	route := cleanRoutePath(c.Args().Get(1))
	key := c.Args().Get(2)

	patchRoute := fnmodels.Route{
		Config: make(map[string]string),
	}

	patchRoute.Config["-"+key] = ""

	err := a.patchRoute(appName, route, &patchRoute)
	if err != nil {
		return err
	}

	fmt.Printf("removed key '%s' from the route '%s%s'", key, appName, key)
	return nil
}

func (a *routesCmd) inspect(c *cli.Context) error {
	appName := c.Args().Get(0)
	route := cleanRoutePath(c.Args().Get(1))
	prop := c.Args().Get(2)

	resp, err := a.client.Routes.GetAppsAppRoutesRoute(&apiroutes.GetAppsAppRoutesRouteParams{
		Context: context.Background(),
		App:     appName,
		Route:   route,
	})

	if err != nil {
		switch err.(type) {
		case *apiroutes.GetAppsAppRoutesRouteNotFound:
			return fmt.Errorf("error: %s", err.(*apiroutes.GetAppsAppRoutesRouteNotFound).Payload.Error.Message)
		case *apiroutes.GetAppsAppRoutesRouteDefault:
			return fmt.Errorf("unexpected error: %s", err.(*apiroutes.GetAppsAppRoutesRouteDefault).Payload.Error.Message)
		}
		return fmt.Errorf("unexpected error: %s", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "\t")

	if prop == "" {
		enc.Encode(resp.Payload.Route)
		return nil
	}

	data, err := json.Marshal(resp.Payload.Route)
	if err != nil {
		return fmt.Errorf("failed to inspect route: %s", err)
	}
	var inspect map[string]interface{}
	err = json.Unmarshal(data, &inspect)
	if err != nil {
		return fmt.Errorf("failed to inspect route: %s", err)
	}

	jq := jsonq.NewQuery(inspect)
	field, err := jq.Interface(strings.Split(prop, ".")...)
	if err != nil {
		return errors.New("failed to inspect that route's field")
	}
	enc.Encode(field)

	return nil
}

func (a *routesCmd) delete(c *cli.Context) error {
	appName := c.Args().Get(0)
	route := cleanRoutePath(c.Args().Get(1))

	_, err := a.client.Routes.DeleteAppsAppRoutesRoute(&apiroutes.DeleteAppsAppRoutesRouteParams{
		Context: context.Background(),
		App:     appName,
		Route:   route,
	})
	if err != nil {
		switch err.(type) {
		case *apiroutes.DeleteAppsAppRoutesRouteNotFound:
			return fmt.Errorf("error: %s", err.(*apiroutes.DeleteAppsAppRoutesRouteNotFound).Payload.Error.Message)
		case *apiroutes.DeleteAppsAppRoutesRouteDefault:
			return fmt.Errorf("unexpected error: %s", err.(*apiroutes.DeleteAppsAppRoutesRouteDefault).Payload.Error.Message)
		}
		return fmt.Errorf("unexpected error: %s", err)
	}

	fmt.Println(appName, route, "deleted")
	return nil
}
