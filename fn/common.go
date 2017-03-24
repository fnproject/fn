package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/iron-io/functions/fn/langs"
)

func verbwriter(verbose bool) io.Writer {
	verbwriter := ioutil.Discard
	if verbose {
		verbwriter = os.Stderr
	}
	return verbwriter
}

func buildfunc(verbwriter io.Writer, fn string) (*funcfile, error) {
	funcfile, err := parsefuncfile(fn)
	if err != nil {
		return nil, err
	}

	if funcfile.Version == "" {
		funcfile, err = bumpversion(*funcfile)
		if err != nil {
			return nil, err
		}
		if err := storefuncfile(fn, funcfile); err != nil {
			return nil, err
		}
		funcfile, err = parsefuncfile(fn)
		if err != nil {
			return nil, err
		}
	}

	if err := localbuild(verbwriter, fn, funcfile.Build); err != nil {
		return nil, err
	}

	if err := dockerbuild(verbwriter, fn, funcfile); err != nil {
		return nil, err
	}

	return funcfile, nil
}

func localbuild(verbwriter io.Writer, path string, steps []string) error {
	for _, cmd := range steps {
		exe := exec.Command("/bin/sh", "-c", cmd)
		exe.Dir = filepath.Dir(path)
		exe.Stderr = verbwriter
		exe.Stdout = verbwriter
		if err := exe.Run(); err != nil {
			return fmt.Errorf("error running command %v (%v)", cmd, err)
		}
	}

	return nil
}

func dockerbuild(verbwriter io.Writer, path string, ff *funcfile) error {
	dir := filepath.Dir(path)

	var helper langs.LangHelper
	dockerfile := filepath.Join(dir, "Dockerfile")
	if !exists(dockerfile) {
		err := writeTmpDockerfile(dir, ff)
		defer os.Remove(filepath.Join(dir, "Dockerfile"))
		if err != nil {
			return err
		}
		helper = langs.GetLangHelper(*ff.Runtime)
		if helper == nil {
			return fmt.Errorf("Cannot build, no language helper found for %v", *ff.Runtime)
		}
		if helper.HasPreBuild() {
			err := helper.PreBuild()
			if err != nil {
				return err
			}
		}
	}

	fmt.Printf("Building image %v\n", ff.FullName())
	cmd := exec.Command("docker", "build", "-t", ff.FullName(), ".")
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running docker build: %v", err)
	}
	if helper != nil {
		err := helper.AfterBuild()
		if err != nil {
			return err
		}
	}
	return nil
}

func exists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

var acceptableFnRuntimes = map[string]string{
	"elixir":           "iron/elixir",
	"erlang":           "iron/erlang",
	"gcc":              "iron/gcc",
	"go":               "iron/go",
	"java":             "iron/java",
	"leiningen":        "iron/leiningen",
	"mono":             "iron/mono",
	"node":             "iron/node",
	"perl":             "iron/perl",
	"php":              "iron/php",
	"python":           "iron/python:2",
	"ruby":             "iron/ruby",
	"scala":            "iron/scala",
	"rust":             "corey/rust-alpine",
	"dotnet":           "microsoft/dotnet:runtime",
	"lambda-nodejs4.3": "iron/functions-lambda:nodejs4.3",
}

const tplDockerfile = `FROM {{ .BaseImage }}
WORKDIR /function
ADD . /function/
{{ if ne .Entrypoint "" }} ENTRYPOINT [{{ .Entrypoint }}] {{ end }}
{{ if ne .Cmd "" }} CMD [{{ .Cmd }}] {{ end }}
`

func writeTmpDockerfile(dir string, ff *funcfile) error {
	if ff.Entrypoint == "" && ff.Cmd == "" {
		return errors.New("entrypoint and cmd are missing, you must provide one or the other")
	}

	runtime, tag := ff.RuntimeTag()
	rt, ok := acceptableFnRuntimes[runtime]
	if !ok {
		return fmt.Errorf("cannot use runtime %s", runtime)
	}

	if tag != "" {
		rt = fmt.Sprintf("%s:%s", rt, tag)
	}

	fd, err := os.Create(filepath.Join(dir, "Dockerfile"))
	if err != nil {
		return err
	}
	defer fd.Close()

	// convert entrypoint string to slice
	bufferEp := stringToSlice(ff.Entrypoint)
	bufferCmd := stringToSlice(ff.Cmd)

	t := template.Must(template.New("Dockerfile").Parse(tplDockerfile))
	err = t.Execute(fd, struct {
		BaseImage, Entrypoint, Cmd string
	}{rt, bufferEp.String(), bufferCmd.String()})

	return err
}

func stringToSlice(in string) bytes.Buffer {
	epvals := strings.Fields(in)
	var buffer bytes.Buffer
	for i, s := range epvals {
		if i > 0 {
			buffer.WriteString(", ")
		}
		buffer.WriteString("\"")
		buffer.WriteString(s)
		buffer.WriteString("\"")
	}
	return buffer
}

func extractEnvConfig(configs []string) map[string]string {
	c := make(map[string]string)
	for _, v := range configs {
		kv := strings.SplitN(v, "=", 2)
		if len(kv) == 2 {
			c[kv[0]] = os.ExpandEnv(kv[1])
		}
	}
	return c
}

func dockerpush(ff *funcfile) error {
	cmd := exec.Command("docker", "push", ff.FullName())
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running docker push: %v", err)
	}
	return nil
}

func appNamePath(img string) (string, string) {
	sep := strings.Index(img, "/")
	if sep < 0 {
		return "", ""
	}
	tag := strings.Index(img[sep:], ":")
	if tag < 0 {
		tag = len(img[sep:])
	}
	return img[:sep], img[sep : sep+tag]
}
