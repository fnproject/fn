package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

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

func (p *publishcmd) walker(path string, info os.FileInfo, err error) error {
	walker(path, info, err, p.publish)
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

	if err := p.dockerpush(funcfile); err != nil {
		return err
	}

	return p.route(path, funcfile)
}

func (p publishcmd) dockerpush(ff *funcfile) error {
	cmd := exec.Command("docker", "push", ff.FullName())
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running docker push: %v", err)
	}
	return nil
}

func (p *publishcmd) route(path string, ff *funcfile) error {
	if err := resetBasePath(p.Configuration); err != nil {
		return fmt.Errorf("error setting endpoint: %v", err)
	}

	// TODO: This is just a nasty hack and should be cleaned up all the way
	pathsSplit := strings.Split(ff.FullName(), "/")

	if ff.App == nil {
		ff.App = &pathsSplit[0]
	}
	if ff.Route == nil {
		path := "/" + strings.Split(pathsSplit[1], ":")[0]
		ff.Route = &path
	}

	if ff.Memory == nil {
		ff.Memory = new(int64)
	}
	if ff.Type == nil {
		ff.Type = new(string)
	}

	body := functions.RouteWrapper{
		Route: functions.Route{
			Path:    *ff.Route,
			Image:   ff.FullName(),
			AppName: *ff.App,
			Memory:  *ff.Memory,
			Type_:   *ff.Type,
			Config:  expandEnvConfig(ff.Config),
		},
	}

	fmt.Fprintf(p.verbwriter, "updating API with app: %s route: %s name: %s \n", *ff.App, *ff.Route, ff.Name)

	wrapper, resp, err := p.AppsAppRoutesPost(*ff.App, body)
	if err != nil {
		return fmt.Errorf("error getting routes: %v", err)
	}
	if resp.StatusCode == http.StatusBadRequest {
		return fmt.Errorf("error storing this route: %s", wrapper.Error_.Message)
	}

	return nil
}

func expandEnvConfig(configs map[string]string) map[string]string {
	for k, v := range configs {
		configs[k] = os.ExpandEnv(v)
	}
	return configs
}

func reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < len(r)/2; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}
