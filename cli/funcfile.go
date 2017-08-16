package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	fnmodels "github.com/funcy/functions_go/models"
	yaml "gopkg.in/yaml.v2"
)

var (
	validfn = [...]string{
		"func.yaml",
		"func.yml",
		"func.json",
	}

	errUnexpectedFileFormat = errors.New("unexpected file format for function file")
)

type inputMap struct {
	Body interface{}
}
type outputMap struct {
	Body interface{}
}

type fftest struct {
	Name   string            `yaml:"name,omitempty" json:"name,omitempty"`
	Input  *inputMap         `yaml:"input,omitempty" json:"input,omitempty"`
	Output *outputMap        `yaml:"outoutput,omitempty" json:"output,omitempty"`
	Err    *string           `yaml:"err,omitempty" json:"err,omitempty"`
	Env    map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

type funcfile struct {
	fnmodels.Route

	Name       string   `yaml:"name,omitempty" json:"name,omitempty"`
	Version    string   `yaml:"version,omitempty" json:"version,omitempty"`
	Runtime    *string  `yaml:"runtime,omitempty" json:"runtime,omitempty"`
	Entrypoint string   `yaml:"entrypoint,omitempty" json:"entrypoint,omitempty"`
	Cmd        string   `yaml:"cmd,omitempty" json:"cmd,omitempty"`
	Build      []string `yaml:"build,omitempty" json:"build,omitempty"`
	Tests      []fftest `yaml:"tests,omitempty" json:"tests,omitempty"`
}

func (ff *funcfile) FullName() string {
	fname := ff.Name
	if ff.Version != "" {
		fname = fmt.Sprintf("%s:%s", fname, ff.Version)
	}
	return fname
}

func (ff *funcfile) RuntimeTag() (runtime, tag string) {
	if ff.Runtime == nil {
		return "", ""
	}

	rt := *ff.Runtime
	tagpos := strings.Index(rt, ":")
	if tagpos == -1 {
		return rt, ""
	}

	return rt[:tagpos], rt[tagpos+1:]
}

type flatfuncfile struct {
	Name       string   `yaml:"name,omitempty" json:"name,omitempty"`
	Version    string   `yaml:"version,omitempty" json:"version,omitempty"`
	Runtime    *string  `yaml:"runtime,omitempty" json:"runtime,omitempty"`
	Entrypoint string   `yaml:"entrypoint,omitempty" json:"entrypoint,omitempty"`
	Cmd        string   `yaml:"cmd,omitempty" json:"cmd,omitempty"`
	Build      []string `yaml:"build,omitempty" json:"build,omitempty"`
	Tests      []fftest `yaml:"tests,omitempty" json:"tests,omitempty"`

	// route specific
	Type    string              `yaml:"type,omitempty" json:"type,omitempty"`
	Memory  uint64              `yaml:"memory,omitempty" json:"memory,omitempty"`
	Format  string              `yaml:"format,omitempty" json:"format,omitempty"`
	Timeout *int32              `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Path    string              `yaml:"path,omitempty" json:"path,omitempty"`
	Config  map[string]string   `yaml:"config,omitempty" json:"config,omitempty"`
	Headers map[string][]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}

func (ff *funcfile) MakeFlat() flatfuncfile {
	return flatfuncfile{
		Name:       ff.Name,
		Version:    ff.Version,
		Runtime:    ff.Runtime,
		Entrypoint: ff.Entrypoint,
		Cmd:        ff.Cmd,
		Build:      ff.Build,
		Tests:      ff.Tests,
		// route-specific
		Type:    ff.Type,
		Memory:  ff.Memory,
		Format:  ff.Format,
		Timeout: ff.Timeout,
		Path:    ff.Path,
		Config:  ff.Config,
		Headers: ff.Headers,
	}
}

func (fff *flatfuncfile) MakeFuncFile() *funcfile {
	ff := &funcfile{
		Name:       fff.Name,
		Version:    fff.Version,
		Runtime:    fff.Runtime,
		Entrypoint: fff.Entrypoint,
		Cmd:        fff.Cmd,
		Build:      fff.Build,
		Tests:      fff.Tests,
	}
	ff.Type = fff.Type
	ff.Memory = fff.Memory
	ff.Format = fff.Format
	ff.Timeout = fff.Timeout
	ff.Path = fff.Path
	ff.Config = fff.Config
	ff.Headers = fff.Headers
	return ff
}

func findFuncfile(path string) (string, error) {
	for _, fn := range validfn {
		fullfn := filepath.Join(path, fn)
		if exists(fullfn) {
			return fullfn, nil
		}
	}
	return "", newNotFoundError("could not find function file")
}

func loadFuncfile() (*funcfile, error) {
	fn, err := findFuncfile(".")
	if err != nil {
		return nil, err
	}
	return parsefuncfile(fn)
}

func parsefuncfile(path string) (*funcfile, error) {
	ext := filepath.Ext(path)
	switch ext {
	case ".json":
		return decodeFuncfileJSON(path)
	case ".yaml", ".yml":
		return decodeFuncfileYAML(path)
	}
	return nil, errUnexpectedFileFormat
}

func storefuncfile(path string, ff *funcfile) error {
	ext := filepath.Ext(path)
	switch ext {
	case ".json":
		return encodeFuncfileJSON(path, ff)
	case ".yaml", ".yml":
		return encodeFuncfileYAML(path, ff)
	}
	return errUnexpectedFileFormat
}

func decodeFuncfileJSON(path string) (*funcfile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open %s for parsing. Error: %v", path, err)
	}
	fff := new(flatfuncfile)
	err = json.NewDecoder(f).Decode(fff)
	ff := fff.MakeFuncFile()
	return ff, err
}

func decodeFuncfileYAML(path string) (*funcfile, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not open %s for parsing. Error: %v", path, err)
	}
	fff := new(flatfuncfile)
	err = yaml.Unmarshal(b, fff)
	ff := fff.MakeFuncFile()
	return ff, err
}

func encodeFuncfileJSON(path string, ff *funcfile) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("could not open %s for encoding. Error: %v", path, err)
	}
	return json.NewEncoder(f).Encode(ff.MakeFlat())
}

func encodeFuncfileYAML(path string, ff *funcfile) error {
	b, err := yaml.Marshal(ff.MakeFlat())
	if err != nil {
		return fmt.Errorf("could not encode function file. Error: %v", err)
	}
	return ioutil.WriteFile(path, b, os.FileMode(0644))
}
