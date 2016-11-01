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

	"github.com/urfave/cli"
	yaml "gopkg.in/yaml.v2"
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
)

type funcfile struct {
	App   *string
	Image string
	Route *string
	Build []string
}

func parsefuncfile(path string) (*funcfile, error) {
	ext := filepath.Ext(path)
	switch ext {
	case ".json":
		return funcfileJSON(path)
	case ".yaml", ".yml":
		return funcfileYAML(path)
	}
	return nil, errUnexpectedFileFormat
}

func funcfileJSON(path string) (*funcfile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open %s for parsing. Error: %v", path, err)
	}
	ff := new(funcfile)
	err = json.NewDecoder(f).Decode(ff)
	return ff, err
}

func funcfileYAML(path string) (*funcfile, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not open %s for parsing. Error: %v", path, err)
	}
	ff := new(funcfile)
	err = yaml.Unmarshal(b, ff)
	return ff, err
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

func walker(path string, info os.FileInfo, err error, w io.Writer, f func(path string) error) {
	if !isvalid(path, info) {
		return
	}

	fmt.Fprint(w, path, "\t")
	if err := f(path); err != nil {
		fmt.Fprintln(w, err)
	} else {
		fmt.Fprintln(w, "done")
	}
}

type commoncmd struct {
	wd      string
	verbose bool

	verbwriter io.Writer
}

func (c *commoncmd) flags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{
			Name:        "d",
			Usage:       "working directory",
			Destination: &c.wd,
			EnvVar:      "WORK_DIR",
			Value:       "./",
		},
		cli.BoolFlag{
			Name:        "v",
			Usage:       "verbose mode",
			Destination: &c.verbose,
		},
	}
}

func (c *commoncmd) scan(walker func(path string, info os.FileInfo, err error, w io.Writer) error) {
	c.verbwriter = ioutil.Discard
	if c.verbose {
		c.verbwriter = os.Stderr
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 0, '\t', 0)
	fmt.Fprint(w, "path", "\t", "result", "\n")

	err := filepath.Walk(c.wd, func(path string, info os.FileInfo, err error) error {
		return walker(path, info, err, w)
	})
	if err != nil {
		fmt.Fprintf(c.verbwriter, "file walk error: %s\n", err)
	}

	w.Flush()
}

func (c commoncmd) buildfunc(path string) (*funcfile, error) {
	dir := filepath.Dir(path)
	dockerfile := filepath.Join(dir, "Dockerfile")
	if _, err := os.Stat(dockerfile); os.IsNotExist(err) {
		return nil, errDockerFileNotFound
	}

	funcfile, err := parsefuncfile(path)
	if err != nil {
		return nil, err
	}

	if err := c.localbuild(path, funcfile.Build); err != nil {
		return nil, err
	}

	if err := c.dockerbuild(path, funcfile.Image); err != nil {
		return nil, err
	}

	return funcfile, nil
}

func (c commoncmd) localbuild(path string, steps []string) error {
	for _, cmd := range steps {
		exe := exec.Command("/bin/sh", "-c", cmd)
		exe.Dir = filepath.Dir(path)
		out, err := exe.CombinedOutput()
		fmt.Fprintf(c.verbwriter, "- %s:\n%s\n", cmd, out)
		if err != nil {
			return fmt.Errorf("error running command %v (%v)", cmd, err)
		}
	}

	return nil
}

func (c commoncmd) dockerbuild(path, image string) error {
	out, err := exec.Command("docker", "build", "-t", image, filepath.Dir(path)).CombinedOutput()
	fmt.Fprintf(c.verbwriter, "%s\n", out)
	if err != nil {
		return fmt.Errorf("error running docker build: %v", err)
	}

	return nil
}
