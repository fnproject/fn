package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"time"

	client "github.com/fnproject/fn/cli/client"
	functions "github.com/funcy/functions_go"
	"github.com/funcy/functions_go/models"
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
		Usage:     "deploys a function to the functions server. (bumps, build, pushes and updates route)",
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
	noCache     bool

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
			// Value:       "./",
		},
		cli.BoolFlag{
			Name:        "i",
			Usage:       "uses incremental building",
			Destination: &p.incremental,
		},
		cli.BoolFlag{
			Name:        "no-cache",
			Usage:       "Don't use Docker cache for the build",
			Destination: &p.noCache,
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
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalln("Couldn't get working directory:", err)
	}

	err = filepath.Walk(wd, func(path string, info os.FileInfo, err error) error {
		if path != wd && info.IsDir() {
			return filepath.SkipDir
		}

		if !isFuncfile(path, info) {
			return nil
		}

		if p.incremental && !isstale(path) {
			return nil
		}

		e := p.deploy(c, path)
		if err != nil {
			fmt.Fprintln(p.verbwriter, path, e)
		}

		now := time.Now()
		os.Chtimes(path, now, now)
		walked = true
		return e
	})
	if err != nil {
		fmt.Fprintf(p.verbwriter, "error: %s\n", err)
	}

	if !walked {
		return errors.New("No function file found.")
	}

	return nil
}

// deploy will perform several actions to deploy to an functions server.
// Parse functions file, bump version, build image, push to registry, and
// finally it will update function's route. Optionally,
// the route can be overriden inside the functions file.
func (p *deploycmd) deploy(c *cli.Context, funcFilePath string) error {
	funcFileName := path.Base(funcFilePath)

	err := c.App.Command("bump").Run(c)
	if err != nil {
		return err
	}

	funcfile, err := buildfunc(p.verbwriter, funcFileName, p.noCache)
	if err != nil {
		return err
	}
	if funcfile.Path == "" {
		funcfile.Path = "/" + path.Base(path.Dir(funcFilePath))
	}

	if p.skippush {
		return nil
	}

	if err := dockerpush(funcfile); err != nil {
		return err
	}

	return p.route(c, funcfile)
}

func (p *deploycmd) route(c *cli.Context, ff *funcfile) error {
	fmt.Printf("Updating route %s using image %s...\n", ff.Path, ff.FullName())
	if err := resetBasePath(p.Configuration); err != nil {
		return fmt.Errorf("error setting endpoint: %v", err)
	}

	routesCmd := routesCmd{client: client.APIClient()}
	rt := &models.Route{}
	if err := routeWithFuncFile(c, ff, rt); err != nil {
		return fmt.Errorf("error getting route with funcfile: %s", err)
	}
	return routesCmd.putRoute(c, p.appName, ff.Path, rt)
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
