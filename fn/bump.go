package main

import (
	"fmt"
	"os"
	"strings"

	bumper "github.com/giantswarm/semver-bump/bump"
	"github.com/giantswarm/semver-bump/storage"
	"github.com/urfave/cli"
)

var (
	initialVersion = "0.0.1"
)

func bump() cli.Command {
	cmd := bumpcmd{commoncmd: &commoncmd{}}
	flags := append([]cli.Flag{}, cmd.flags()...)
	return cli.Command{
		Name:   "bump",
		Usage:  "bump function version",
		Flags:  flags,
		Action: cmd.scan,
	}
}

type bumpcmd struct {
	*commoncmd
}

func (b *bumpcmd) scan(c *cli.Context) error {
	b.commoncmd.scan(b.walker)
	return nil
}

func (b *bumpcmd) walker(path string, info os.FileInfo, err error) error {
	walker(path, info, err, b.bump)
	return nil
}

// bump will take the found valid function and bump its version
func (b *bumpcmd) bump(path string) error {
	fmt.Fprintln(b.verbwriter, "bumping version for", path)

	funcfile, err := parsefuncfile(path)
	if err != nil {
		return err
	}

	funcfile.Name = cleanImageName(funcfile.Name)
	if funcfile.Version == "" {
		funcfile.Version = initialVersion
	}

	s, err := storage.NewVersionStorage("local", funcfile.Version)
	if err != nil {
		return err
	}

	version := bumper.NewSemverBumper(s, "")
	newver, err := version.BumpPatchVersion("", "")
	if err != nil {
		return err
	}

	funcfile.Version = newver.String()

	if err := storefuncfile(path, funcfile); err != nil {
		return err
	}

	fmt.Println("Bumped to version", funcfile.Version)
	return nil
}

func cleanImageName(name string) string {
	if i := strings.Index(name, ":"); i != -1 {
		name = name[:i]
	}

	return name
}
