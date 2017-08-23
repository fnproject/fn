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
	cmd := bumpcmd{}
	flags := append([]cli.Flag{}, cmd.flags()...)
	return cli.Command{
		Name:   "bump",
		Usage:  "bump function version",
		Flags:  flags,
		Action: cmd.bump,
	}
}

type bumpcmd struct {
	verbose bool
}

func (b *bumpcmd) flags() []cli.Flag {
	return []cli.Flag{
		cli.BoolFlag{
			Name:        "v",
			Usage:       "verbose mode",
			Destination: &b.verbose,
		},
	}
}

// bump will take the found valid function and bump its version
func (b *bumpcmd) bump(c *cli.Context) error {

	path, err := os.Getwd()
	if err != nil {
		return err
	}
	fn, err := findFuncfile(path)
	if err != nil {
		return err
	}

	fmt.Println("bumping version for", fn)

	funcfile, err := parsefuncfile(fn)
	if err != nil {
		return err
	}

	funcfile, err = bumpversion(*funcfile)
	if err != nil {
		return err
	}

	if err := storefuncfile(fn, funcfile); err != nil {
		return err
	}

	fmt.Println("Bumped to version", funcfile.Version)
	return nil
}

func bumpversion(funcfile funcfile) (*funcfile, error) {
	funcfile.Name = cleanImageName(funcfile.Name)
	if funcfile.Version == "" {
		funcfile.Version = initialVersion
		return &funcfile, nil
	}

	s, err := storage.NewVersionStorage("local", funcfile.Version)
	if err != nil {
		return nil, err
	}

	version := bumper.NewSemverBumper(s, "")
	newver, err := version.BumpPatchVersion("", "")
	if err != nil {
		return nil, err
	}

	funcfile.Version = newver.String()
	return &funcfile, nil
}

func cleanImageName(name string) string {
	if i := strings.Index(name, ":"); i != -1 {
		name = name[:i]
	}

	return name
}
