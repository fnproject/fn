package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	functions "github.com/iron-io/functions_go"
	"github.com/urfave/cli"
)

func deploy() cli.Command {
	cmd := deploycmd{
		RoutesApi: functions.NewRoutesApi(),
	}
	var flags []cli.Flag
	flags = append(flags, cmd.flags()...)
	return cli.Command{
		Name:      "deploy",
		ArgsUsage: "<appName>",
		Usage:     "scan local directory for functions, build and push all of them to `APPNAME`.",
		Flags:     flags,
		Action:    cmd.scan,
	}
}

type deploycmd struct {
	appName string
	*functions.RoutesApi

	wd          string
	verbose     bool
	incremental bool
	skippush    bool

	verbwriter io.Writer
}

func (p *deploycmd) flags() []cli.Flag {
	return []cli.Flag{
		cli.BoolFlag{
			Name:        "v",
			Usage:       "verbose mode",
			Destination: &p.verbose,
		},
		cli.StringFlag{
			Name:        "d",
			Usage:       "working directory",
			Destination: &p.wd,
			EnvVar:      "WORK_DIR",
			Value:       "./",
		},
		cli.BoolFlag{
			Name:        "i",
			Usage:       "uses incremental building",
			Destination: &p.incremental,
		},
		cli.BoolFlag{
			Name:        "skip-push",
			Usage:       "does not push Docker built images onto Docker Hub - useful for local development.",
			Destination: &p.skippush,
		},
	}
}

func (p *deploycmd) scan(c *cli.Context) error {
	p.appName = c.Args().First()
	p.verbwriter = verbwriter(p.verbose)

	var walked bool

	err := filepath.Walk(p.wd, func(path string, info os.FileInfo, err error) error {
		if path != p.wd && info.IsDir() {
			return filepath.SkipDir
		}

		if !isFuncfile(path, info) {
			return nil
		}

		if p.incremental && !isstale(path) {
			return nil
		}

		e := p.deploy(path)
		if err != nil {
			fmt.Fprintln(p.verbwriter, path, e)
		}

		now := time.Now()
		os.Chtimes(path, now, now)
		walked = true
		return e
	})
	if err != nil {
		fmt.Fprintf(p.verbwriter, "file walk error: %s\n", err)
	}

	if !walked {
		return errors.New("No function file found.")
	}

	return nil
}

// deploy will take the found function and check for the presence of a
// Dockerfile, and run a three step process: parse functions file, build and
// push the container, and finally it will update function's route. Optionally,
// the route can be overriden inside the functions file.
func (p *deploycmd) deploy(path string) error {
	fmt.Fprintln(p.verbwriter, "deploying", path)

	funcfile, err := buildfunc(p.verbwriter, path)
	if err != nil {
		return err
	}

	if p.skippush {
		return nil
	}

	if err := dockerpush(funcfile); err != nil {
		return err
	}

	return p.route(path, funcfile)
}

func (p *deploycmd) route(path string, ff *funcfile) error {
	if err := resetBasePath(p.Configuration); err != nil {
		return fmt.Errorf("error setting endpoint: %v", err)
	}

	if ff.Path == nil {
		_, path := appNamePath(ff.FullName())
		ff.Path = &path
	}

	if ff.Memory == nil {
		ff.Memory = new(int64)
	}
	if ff.Type == nil {
		ff.Type = new(string)
	}
	if ff.Format == nil {
		ff.Format = new(string)
	}
	if ff.MaxConcurrency == nil {
		ff.MaxConcurrency = new(int)
	}
	if ff.Timeout == nil {
		dur := time.Duration(0)
		ff.Timeout = &dur
	}

	headers := make(map[string][]string)
	for k, v := range ff.Headers {
		headers[k] = []string{v}
	}
	body := functions.RouteWrapper{
		Route: functions.Route{
			Path:           *ff.Path,
			Image:          ff.FullName(),
			Memory:         *ff.Memory,
			Type_:          *ff.Type,
			Config:         expandEnvConfig(ff.Config),
			Headers:        headers,
			Format:         *ff.Format,
			MaxConcurrency: int32(*ff.MaxConcurrency),
			Timeout:        int32(ff.Timeout.Seconds()),
		},
	}

	fmt.Fprintf(p.verbwriter, "updating API with app: %s route: %s name: %s \n", p.appName, *ff.Path, ff.Name)

	wrapper, resp, err := p.AppsAppRoutesPost(p.appName, body)
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

func isFuncfile(path string, info os.FileInfo) bool {
	if info.IsDir() {
		return false
	}

	basefn := filepath.Base(path)
	for _, fn := range validfn {
		if basefn == fn {
			return true
		}
	}

	return false
}

// Theory of operation: this takes an optimistic approach to detect whether a
// package must be rebuild/bump/deployed. It loads for all files mtime's and
// compare with functions.json own mtime. If any file is younger than
// functions.json, it triggers a rebuild.
// The problem with this approach is that depending on the OS running it, the
// time granularity of these timestamps might lead to false negatives - that is
// a package that is stale but it is not recompiled. A more elegant solution
// could be applied here, like https://golang.org/src/cmd/go/pkg.go#L1111
func isstale(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return true
	}

	fnmtime := fi.ModTime()
	dir := filepath.Dir(path)
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if info.ModTime().After(fnmtime) {
			return errors.New("found stale package")
		}
		return nil
	})

	return err != nil
}
