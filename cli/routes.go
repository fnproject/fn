package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"
	"text/tabwriter"

	fnclient "github.com/funcy/functions_go/client"
	apiroutes "github.com/funcy/functions_go/client/routes"
	fnmodels "github.com/funcy/functions_go/models"
	"github.com/jmoiron/jsonq"
	"github.com/urfave/cli"
	client "gitlab-odx.oracle.com/odx/functions/fn/client"
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
	cli.DurationFlag{
		Name:  "timeout",
		Usage: "route timeout (eg. 30s)",
	},
}

func routes() cli.Command {

	r := routesCmd{client: client.APIClient()}

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
	r := routesCmd{client: client.APIClient()}

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

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', 0)
	fmt.Fprint(w, "path", "\t", "image", "\t", "endpoint", "\n")
	for _, route := range resp.Payload.Routes {
		endpoint := path.Join(client.Host(), "r", appName, route.Path)
		if err != nil {
			return fmt.Errorf("error parsing functions route path: %s", err)
		}

		fmt.Fprint(w, route.Path, "\t", route.Image, "\t", endpoint, "\n")
	}
	w.Flush()

	return nil
}

func (a *routesCmd) call(c *cli.Context) error {
	appName := c.Args().Get(0)
	route := cleanRoutePath(c.Args().Get(1))

	u := url.URL{
		Scheme: "http",
		Host:   client.Host(),
	}
	u.Path = path.Join(u.Path, "r", appName, route)
	content := stdin()

	return client.CallFN(u.String(), content, os.Stdout, c.String("method"), c.StringSlice("e"))
}

func routeWithFlags(c *cli.Context, rt *fnmodels.Route) {
	if i := c.String("image"); i != "" {
		rt.Image = i
	}

	if f := c.String("format"); f != "" {
		rt.Format = f
	}

	if t := c.String("type"); t != "" {
		rt.Type = t
	}

	if m := c.Int64("memory"); m > 0 {
		rt.Memory = m
	}

	if t := c.Duration("timeout"); t > 0 {
		to := int32(t.Seconds())
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

func routeWithFuncFile(c *cli.Context, ff *funcfile, rt *fnmodels.Route) error {
	var err error
	if ff == nil {
		ff, err = loadFuncfile()
		if err != nil {
			return err
		}
	}
	if ff.FullName() != "" { // args take precedence
		rt.Image = ff.FullName()
	}
	if ff.Format != nil {
		rt.Format = *ff.Format
	}
	if ff.Timeout != nil {
		to := int32(ff.Timeout.Seconds())
		rt.Timeout = &to
	}
	if rt.Path == "" && ff.Path != "" {
		rt.Path = ff.Path
	}
	if rt.Type == "" && ff.Type != nil && *ff.Type != "" {
		rt.Type = *ff.Type
	}

	return nil
}

func (a *routesCmd) create(c *cli.Context) error {
	appName := c.Args().Get(0)
	route := cleanRoutePath(c.Args().Get(1))

	rt := &fnmodels.Route{}
	rt.Path = route
	rt.Image = c.Args().Get(2)

	if err := routeWithFuncFile(c, nil, rt); err != nil {
		return fmt.Errorf("error getting route info: %s", err)
	}

	routeWithFlags(c, rt)

	if rt.Path == "" {
		return errors.New("route path is missing")
	}
	if rt.Image == "" {
		return errors.New("no image specified")
	}

	return a.postRoute(c, appName, rt)
}

func (a *routesCmd) postRoute(c *cli.Context, appName string, rt *fnmodels.Route) error {

	body := &fnmodels.RouteWrapper{
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

func (a *routesCmd) patchRoute(c *cli.Context, appName, routePath string, r *fnmodels.Route) error {
	_, err := a.client.Routes.PatchAppsAppRoutesRoute(&apiroutes.PatchAppsAppRoutesRouteParams{
		Context: context.Background(),
		App:     appName,
		Route:   routePath,
		Body:    &fnmodels.RouteWrapper{Route: r},
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

func (a *routesCmd) putRoute(c *cli.Context, appName, routePath string, r *fnmodels.Route) error {
	_, err := a.client.Routes.PutAppsAppRoutesRoute(&apiroutes.PutAppsAppRoutesRouteParams{
		Context: context.Background(),
		App:     appName,
		Route:   routePath,
		Body:    &fnmodels.RouteWrapper{Route: r},
	})
	if err != nil {
		switch err.(type) {
		case *apiroutes.PutAppsAppRoutesRouteBadRequest:
			return fmt.Errorf("error: %s", err.(*apiroutes.PutAppsAppRoutesRouteBadRequest).Payload.Error.Message)
		case *apiroutes.PutAppsAppRoutesRouteDefault:
			return fmt.Errorf("unexpected error: %s", err.(*apiroutes.PutAppsAppRoutesRouteDefault).Payload.Error.Message)
		}
		return fmt.Errorf("unexpected error: %s", err)
	}
	return nil
}

func (a *routesCmd) update(c *cli.Context) error {
	appName := c.Args().Get(0)
	route := cleanRoutePath(c.Args().Get(1))

	rt := &fnmodels.Route{}

	if err := routeWithFuncFile(c, nil, rt); err != nil {
		return fmt.Errorf("error updating route: %s", err)
	}

	routeWithFlags(c, rt)

	err := a.patchRoute(c, appName, route, rt)
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

	err := a.patchRoute(c, appName, route, &patchRoute)
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

	patchRoute.Config[key] = ""

	err := a.patchRoute(c, appName, route, &patchRoute)
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
