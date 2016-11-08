package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	functions "github.com/iron-io/functions_go"
	"github.com/urfave/cli"
)

func publish() cli.Command {
	cmd := publishcmd{
		commoncmd: &commoncmd{},
		RoutesApi: functions.NewRoutesApi(),
	}
	var flags []cli.Flag
	flags = append(flags, cmd.flags()...)
	flags = append(flags, cmd.commoncmd.flags()...)
	flags = append(flags, confFlags(&cmd.Configuration)...)
	return cli.Command{
		Name:   "publish",
		Usage:  "scan local directory for functions, build and publish them.",
		Flags:  flags,
		Action: cmd.scan,
	}
}

type publishcmd struct {
	*commoncmd
	*functions.RoutesApi

	skippush bool
}

func (p *publishcmd) flags() []cli.Flag {
	return []cli.Flag{
		cli.BoolFlag{
			Name:        "skip-push",
			Usage:       "does not push Docker built images onto Docker Hub - useful for local development.",
			Destination: &p.skippush,
		},
	}
}

func (p *publishcmd) scan(c *cli.Context) error {
	p.commoncmd.scan(p.walker)
	return nil
}

func (p *publishcmd) walker(path string, info os.FileInfo, err error, w io.Writer) error {
	walker(path, info, err, w, p.publish)
	return nil
}

// publish will take the found function and check for the presence of a
// Dockerfile, and run a three step process: parse functions file, build and
// push the container, and finally it will update function's route. Optionally,
// the route can be overriden inside the functions file.
func (p *publishcmd) publish(path string) error {
	fmt.Fprintln(p.verbwriter, "publishing", path)

	funcfile, err := p.buildfunc(path)
	if err != nil {
		return err
	}

	if p.skippush {
		return nil
	}

	if err := p.dockerpush(funcfile.Image); err != nil {
		return err
	}

	if err := p.route(path, funcfile); err != nil {
		return err
	}

	return nil
}

func (p publishcmd) dockerpush(image string) error {
	cmd := exec.Command("docker", "push", image)
	cmd.Stderr = p.verbwriter
	cmd.Stdout = p.verbwriter
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running docker push: %v", err)
	}
	return nil
}

func (p *publishcmd) route(path string, ff *funcfile) error {
	if err := resetBasePath(&p.Configuration); err != nil {
		return fmt.Errorf("error setting endpoint: %v", err)
	}

	an, r := extractAppNameRoute(path)
	if ff.App == nil {
		ff.App = &an
	}
	if ff.Route == nil {
		ff.Route = &r
	}

	body := functions.RouteWrapper{
		Route: functions.Route{
			Path:   *ff.Route,
			Image:  ff.Image,
			Memory: ff.Memory,
			Type_:  ff.Type,
			Config: ff.Config,
		},
	}

	fmt.Fprintf(p.verbwriter, "updating API with appName: %s route: %s image: %s \n", *ff.App, *ff.Route, ff.Image)

	wrapper, resp, err := p.AppsAppRoutesPost(*ff.App, body)
	if err != nil {
		return fmt.Errorf("error getting routes: %v", err)
	}
	if resp.StatusCode == http.StatusBadRequest {
		return fmt.Errorf("error storing this route: %s", wrapper.Error_.Message)
	}

	return nil
}

func extractAppNameRoute(path string) (appName, route string) {

	// The idea here is to extract the root-most directory name
	// as application name, it turns out that stdlib tools are great to
	// extract the deepest one. Thus, we revert the string and use the
	// stdlib as it is - and revert back to its normal content. Not fastest
	// ever, but it is simple.

	rpath := reverse(path)
	rroute, rappName := filepath.Split(rpath)
	route = filepath.Dir(reverse(rroute))
	return reverse(rappName), route
}

func reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < len(r)/2; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}
