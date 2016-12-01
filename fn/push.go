package main

import (
	"errors"
	"fmt"

	"github.com/urfave/cli"
)

func push() cli.Command {
	cmd := pushcmd{}
	var flags []cli.Flag
	flags = append(flags, cmd.flags()...)
	return cli.Command{
		Name:   "push",
		Usage:  "push function to Docker Hub",
		Flags:  flags,
		Action: cmd.push,
	}
}

type pushcmd struct {
	verbose bool
}

func (p *pushcmd) flags() []cli.Flag {
	return []cli.Flag{
		cli.BoolFlag{
			Name:        "v",
			Usage:       "verbose mode",
			Destination: &p.verbose,
		},
	}
}

// push will take the found function and check for the presence of a
// Dockerfile, and run a three step process: parse functions file,
// push the container, and finally it will update function's route. Optionally,
// the route can be overriden inside the functions file.
func (p *pushcmd) push(c *cli.Context) error {
	verbwriter := verbwriter(p.verbose)

	ff, err := loadFuncfile()
	if err != nil {
		if _, ok := err.(*notFoundError); ok {
			return errors.New("error: image name is missing or no function file found")
		}
		return err
	}

	fmt.Fprintln(verbwriter, "pushing", ff.FullName())

	if err := dockerpush(ff); err != nil {
		return err
	}

	fmt.Printf("Function %v pushed successfully to Docker Hub.\n", ff.FullName())
	return nil
}
