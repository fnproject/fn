package main

import (
	"fmt"
	"io"
	"os"

	functions "github.com/iron-io/functions_go"
	"github.com/urfave/cli"
)

func push() cli.Command {
	cmd := pushcmd{
		publishcmd: &publishcmd{
			commoncmd: &commoncmd{},
			RoutesApi: functions.NewRoutesApi(),
		},
	}
	var flags []cli.Flag
	flags = append(flags, cmd.commoncmd.flags()...)
	return cli.Command{
		Name:   "push",
		Usage:  "push function to Docker Hub",
		Flags:  flags,
		Action: cmd.scan,
	}
}

type pushcmd struct {
	*publishcmd
}

func (p *pushcmd) scan(c *cli.Context) error {
	p.commoncmd.scan(p.walker)
	return nil
}

func (p *pushcmd) walker(path string, info os.FileInfo, err error, w io.Writer) error {
	walker(path, info, err, w, p.push)
	return nil
}

// push will take the found function and check for the presence of a
// Dockerfile, and run a three step process: parse functions file,
// push the container, and finally it will update function's route. Optionally,
// the route can be overriden inside the functions file.
func (p *pushcmd) push(path string) error {
	fmt.Fprintln(p.verbwriter, "pushing", path)

	funcfile, err := parsefuncfile(path)
	if err != nil {
		return err
	}

	if err := p.dockerpush(funcfile); err != nil {
		return err
	}
	return nil
}
