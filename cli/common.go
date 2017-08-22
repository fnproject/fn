package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/coreos/go-semver/semver"

	"github.com/fnproject/fn/cli/langs"
)

const (
	functionsDockerImage     = "fnproject/functions"
	minRequiredDockerVersion = "17.5.0"
)

func verbwriter(verbose bool) io.Writer {
	// this is too limiting, removes all logs which isn't what we want
	// verbwriter := ioutil.Discard
	// if verbose {
	verbwriter := os.Stderr
	// }
	return verbwriter
}

func buildfunc(verbwriter io.Writer, fn string, noCache bool) (*funcfile, error) {
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

	if err := dockerbuild(verbwriter, fn, funcfile, noCache); err != nil {
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

func dockerbuild(verbwriter io.Writer, path string, ff *funcfile, noCache bool) error {
	err := dockerVersionCheck()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)

	var helper langs.LangHelper
	dockerfile := filepath.Join(dir, "Dockerfile")
	if !exists(dockerfile) {
		helper = langs.GetLangHelper(*ff.Runtime)
		if helper == nil {
			return fmt.Errorf("Cannot build, no language helper found for %v", *ff.Runtime)
		}
		dockerfile, err = writeTmpDockerfile(helper, dir, ff)
		if err != nil {
			return err
		}
		defer os.Remove(dockerfile)
		if helper.HasPreBuild() {
			err := helper.PreBuild()
			if err != nil {
				return err
			}
		}
	}

	fmt.Printf("Building image %v\n", ff.FullName())

	cancel := make(chan os.Signal, 3)
	signal.Notify(cancel, os.Interrupt) // and others perhaps
	defer signal.Stop(cancel)

	result := make(chan error, 1)

	go func(done chan<- error) {
		args := []string{
			"build",
			"-t", ff.FullName(),
			"-f", dockerfile,
		}
		if noCache {
			args = append(args, "--no-cache")
		}
		args = append(args,
			"--build-arg", "HTTP_PROXY",
			"--build-arg", "HTTPS_PROXY",
			".")
		cmd := exec.Command("docker", args...)
		cmd.Dir = dir
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		done <- cmd.Run()
	}(result)

	select {
	case err := <-result:
		if err != nil {
			return fmt.Errorf("error running docker build: %v", err)
		}
	case signal := <-cancel:
		return fmt.Errorf("build cancelled on signal %v", signal)
	}

	if helper != nil {
		err := helper.AfterBuild()
		if err != nil {
			return err
		}
	}
	return nil
}

func dockerVersionCheck() error {
	out, err := exec.Command("docker", "version", "--format", "{{.Server.Version}}").Output()
	if err != nil {
		return fmt.Errorf("could not check Docker version: %v", err)
	}
	// dev / test builds append '-ce', trim this
	trimmed := strings.TrimRightFunc(string(out), func(r rune) bool { return r != '.' && !unicode.IsDigit(r) })

	v, err := semver.NewVersion(trimmed)
	if err != nil {
		return fmt.Errorf("could not check Docker version: %v", err)
	}
	vMin, err := semver.NewVersion(minRequiredDockerVersion)
	if err != nil {
		return fmt.Errorf("our bad, sorry... please make an issue.", err)
	}
	if v.LessThan(*vMin) {
		return fmt.Errorf("please upgrade your version of Docker to %s or greater", minRequiredDockerVersion)
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

func writeTmpDockerfile(helper langs.LangHelper, dir string, ff *funcfile) (string, error) {
	if ff.Entrypoint == "" && ff.Cmd == "" {
		return "", errors.New("entrypoint and cmd are missing, you must provide one or the other")
	}

	fd, err := ioutil.TempFile(dir, "Dockerfile")
	if err != nil {
		return "", err
	}
	defer fd.Close()

	// multi-stage build: https://medium.com/travis-on-docker/multi-stage-docker-builds-for-creating-tiny-go-images-e0e1867efe5a
	dfLines := []string{}
	if helper.IsMultiStage() {
		// build stage
		dfLines = append(dfLines, fmt.Sprintf("FROM %s as build-stage", helper.BuildFromImage()))
	} else {
		dfLines = append(dfLines, fmt.Sprintf("FROM %s", helper.BuildFromImage()))
	}
	dfLines = append(dfLines, "WORKDIR /function")
	dfLines = append(dfLines, helper.DockerfileBuildCmds()...)
	if helper.IsMultiStage() {
		// final stage
		dfLines = append(dfLines, fmt.Sprintf("FROM %s", helper.RunFromImage()))
		dfLines = append(dfLines, "WORKDIR /function")
		dfLines = append(dfLines, helper.DockerfileCopyCmds()...)
	}
	if ff.Entrypoint != "" {
		dfLines = append(dfLines, fmt.Sprintf("ENTRYPOINT [%s]", stringToSlice(ff.Entrypoint)))
	}
	if ff.Cmd != "" {
		dfLines = append(dfLines, fmt.Sprintf("CMD [%s]", stringToSlice(ff.Cmd)))
	}
	err = writeLines(fd, dfLines)
	if err != nil {
		return "", err
	}
	return fd.Name(), err
}

func writeLines(w io.Writer, lines []string) error {
	writer := bufio.NewWriter(w)
	for _, l := range lines {
		_, err := writer.WriteString(l + "\n")
		if err != nil {
			return err
		}
	}
	writer.Flush()
	return nil
}

func stringToSlice(in string) string {
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
	return buffer.String()
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
	fmt.Println("Pushing to docker registry...")
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
