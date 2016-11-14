package main

import (
	"fmt"
	"io"
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

func (b *bumpcmd) walker(path string, info os.FileInfo, err error, w io.Writer) error {
	walker(path, info, err, w, b.bump)
	return nil
}

// bump will take the found valid function and bump its version
func (b *bumpcmd) bump(path string) error {
	fmt.Fprintln(b.verbwriter, "bumping version for", path)

	funcfile, err := parsefuncfile(path)
	if err != nil {
		return err
	}

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

	err = storefuncfile(path, funcfile)
	if err != nil {
		return err
	}
	fmt.Println("Bumped to version", funcfile.Version)
	return nil
}

func imageversion(image string) (name, ver string) {
	tagpos := strings.Index(image, ":")
	if tagpos == -1 {
		return image, initialVersion
	}

	imgname, imgver := image[:tagpos], image[tagpos+1:]

	s, err := storage.NewVersionStorage("local", imgver)
	if err != nil {
		return imgname, ""
	}

	version := bumper.NewSemverBumper(s, "")
	v, err := version.GetCurrentVersion()
	if err != nil {
		return imgname, ""
	}

	return imgname, v.String()
}
