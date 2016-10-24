package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"text/tabwriter"

	functions "github.com/iron-io/functions_go"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

var (
	validfn = [...]string{
		"functions.yaml",
		"functions.yml",
		"fn.yaml",
		"fn.yml",
		"functions.json",
		"fn.json",
	}

	errDockerFileNotFound   = errors.New("no Dockerfile found for this function")
	errUnexpectedFileFormat = errors.New("unexpected file format for function file")
	verbwriter              = ioutil.Discard
)

func update() cli.Command {
	cmd := updatecmd{RoutesApi: functions.NewRoutesApi()}
	var flags []cli.Flag
	flags = append(flags, cmd.flags()...)
	flags = append(flags, confFlags(&cmd.Configuration)...)
	return cli.Command{
		Name:   "update",
		Usage:  "scan local directory for functions, build and update them.",
		Flags:  flags,
		Action: cmd.scan,
	}
}

type updatecmd struct {
	*functions.RoutesApi

	wd       string
	dry      bool
	skippush bool
	verbose  bool
}

func (u *updatecmd) flags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{
			Name:        "d",
			Usage:       "working directory",
			Destination: &u.wd,
			EnvVar:      "WORK_DIR",
			Value:       "./",
		},
		cli.BoolFlag{
			Name:        "skip-push",
			Usage:       "does not push Docker built images onto Docker Hub - useful for local development.",
			Destination: &u.skippush,
		},
		cli.BoolFlag{
			Name:        "dry-run",
			Usage:       "display how update will proceed when executed",
			Destination: &u.dry,
		},
		cli.BoolFlag{
			Name:        "v",
			Usage:       "verbose mode",
			Destination: &u.verbose,
		},
	}
}

func (u *updatecmd) scan(c *cli.Context) error {
	if u.verbose {
		verbwriter = os.Stderr
	}

	os.Chdir(u.wd)

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 0, '\t', 0)
	fmt.Fprint(w, "path", "\t", "action", "\n")

	filepath.Walk(u.wd, func(path string, info os.FileInfo, err error) error {
		return u.walker(path, info, err, w)
	})

	w.Flush()
	return nil
}

func (u *updatecmd) walker(path string, info os.FileInfo, err error, w io.Writer) error {
	if !isvalid(path, info) {
		return nil
	}

	fmt.Fprint(w, path, "\t")
	if u.dry {
		fmt.Fprintln(w, "dry-run")
	} else if err := u.update(path); err != nil {
		fmt.Fprintln(w, err)
	} else {
		fmt.Fprintln(w, "updated")
	}

	return nil
}

func isvalid(path string, info os.FileInfo) bool {
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

// update will take the found function and check for the presence of a Dockerfile,
// and run a three step process: parse functions file, build and push the
// container, and finally it will update function's route. Optionally, the route
// can be overriden inside the functions file.
func (u *updatecmd) update(path string) error {
	fmt.Fprintln(verbwriter, "deploying", path)

	dir := filepath.Dir(path)
	dockerfile := filepath.Join(dir, "Dockerfile")
	if _, err := os.Stat(dockerfile); os.IsNotExist(err) {
		return errDockerFileNotFound
	}

	funcfile, err := u.parse(path)
	if err != nil {
		return err
	}

	if funcfile.Build != nil {
		if err := u.localbuild(path, funcfile.Build); err != nil {
			return err
		}
	}
	if err := u.dockerbuild(path, funcfile.Image); err != nil {
		return err
	}

	if err := u.route(path, funcfile); err != nil {
		return err
	}

	return nil
}

func (u *updatecmd) parse(path string) (*funcfile, error) {
	ext := filepath.Ext(path)
	switch ext {
	case ".json":
		return parseJSON(path)
	case ".yaml", ".yml":
		return parseYAML(path)
	}
	return nil, errUnexpectedFileFormat
}

func parseJSON(path string) (*funcfile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open %s for parsing. Error: %v", path, err)
	}
	ff := new(funcfile)
	err = json.NewDecoder(f).Decode(ff)
	return ff, err
}

func parseYAML(path string) (*funcfile, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not open %s for parsing. Error: %v", path, err)
	}
	ff := new(funcfile)
	err = yaml.Unmarshal(b, ff)
	return ff, err
}

type funcfile struct {
	App   *string
	Image string
	Route *string
	Build []string
}

func (u *updatecmd) localbuild(path string, steps []string) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot get current working directory. err: %v", err)
	}

	fullwd := filepath.Join(wd, filepath.Dir(path))
	for _, cmd := range steps {
		c := exec.Command("/bin/sh", "-c", cmd)
		c.Dir = fullwd
		out, err := c.CombinedOutput()
		fmt.Fprintf(verbwriter, "- %s:\n%s\n", cmd, out)
		if err != nil {
			return fmt.Errorf("error running command %v (%v)", cmd, err)
		}
	}

	return nil
}

func (u *updatecmd) dockerbuild(path, image string) error {
	cmds := [][]string{
		{"docker", "build", "-t", image, filepath.Dir(path)},
	}
	if !u.skippush {
		cmds = append(cmds, []string{"docker", "push", image})
	}

	for _, cmd := range cmds {
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		fmt.Fprintf(verbwriter, "%s\n", out)
		if err != nil {
			return fmt.Errorf("error running command %v (%v)", cmd, err)
		}
	}

	return nil
}

func (u *updatecmd) route(path string, ff *funcfile) error {
	resetBasePath(&u.Configuration)

	an, r := extractAppNameRoute(path)
	if ff.App == nil {
		ff.App = &an
	}
	if ff.Route == nil {
		ff.Route = &r
	}

	body := functions.RouteWrapper{
		Route: functions.Route{
			Path:  *ff.Route,
			Image: ff.Image,
		},
	}

	fmt.Fprintf(verbwriter, "updating API with appName: %s route: %s image: %s \n", *ff.App, *ff.Route, ff.Image)

	_, _, err := u.AppsAppRoutesPost(*ff.App, body)
	if err != nil {
		return fmt.Errorf("error getting routes: %v", err)
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
