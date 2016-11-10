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
	"strings"
	"text/tabwriter"
	"time"

	"github.com/urfave/cli"
	yaml "gopkg.in/yaml.v2"
)

var (
	validfn = [...]string{
		"functions.yaml",
		"functions.yml",
		"function.yaml",
		"function.yml",
		"fn.yaml",
		"fn.yml",
		"functions.json",
		"function.json",
		"fn.json",
	}

	errDockerFileNotFound   = errors.New("no Dockerfile found for this function")
	errUnexpectedFileFormat = errors.New("unexpected file format for function file")
)

type funcfile struct {
	App    *string
	Image  string
	Route  *string
	Type   string
	Memory int64
	Config map[string]string
	Build  []string
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
	fmt.Fprint(w, path, "\t")
	if err := f(path); err != nil {
		fmt.Fprintln(w, err)
	} else {
		fmt.Fprintln(w, "done")
	}
}

type commoncmd struct {
	wd          string
	verbose     bool
	force       bool
	recursively bool

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
		cli.BoolFlag{
			Name:        "f",
			Usage:       "force updating of all functions that are already up-to-date",
			Destination: &c.force,
		},
		cli.BoolFlag{
			Name:        "r",
			Usage:       "recursively scan all functions",
			Destination: &c.recursively,
		},
	}
}

func (c *commoncmd) scan(walker func(path string, info os.FileInfo, err error, w io.Writer) error) {
	c.verbwriter = ioutil.Discard
	if c.verbose {
		c.verbwriter = os.Stderr
	}

	var walked bool

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 0, '\t', 0)
	fmt.Fprint(w, "path", "\t", "result", "\n")

	err := filepath.Walk(c.wd, func(path string, info os.FileInfo, err error) error {

		if !c.recursively && path != c.wd && info.IsDir() {
			return filepath.SkipDir
		}

		if !isvalid(path, info) {
			return nil
		}

		if c.recursively && !c.force && !isstale(path) {
			return nil
		}

		e := walker(path, info, err, w)
		now := time.Now()
		os.Chtimes(path, now, now)
		walked = true
		return e
	})
	if err != nil {
		fmt.Fprintf(c.verbwriter, "file walk error: %s\n", err)
	}

	if !walked {
		fmt.Println("all functions are up-to-date.")
		return
	}

	w.Flush()
}

// Theory of operation: this takes an optimistic approach to detect whether a
// package must be rebuild/bump/published. It loads for all files mtime's and
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
		exe.Stderr = c.verbwriter
		exe.Stdout = c.verbwriter
		fmt.Fprintf(c.verbwriter, "- %s:\n", cmd)
		if err := exe.Run(); err != nil {
			return fmt.Errorf("error running command %v (%v)", cmd, err)
		}
	}

	return nil
}

func (c commoncmd) dockerbuild(path, image string) error {
	cmd := exec.Command("docker", "build", "-t", image, filepath.Dir(path))
	cmd.Stderr = c.verbwriter
	cmd.Stdout = c.verbwriter
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running docker build: %v", err)
	}

	return nil
}

func extractEnvConfig(configs []string) map[string]string {
	c := make(map[string]string)
	for _, v := range configs {
		kv := strings.SplitN(v, "=", 2)
		c[kv[0]] = os.ExpandEnv(kv[1])
	}
	return c
}
