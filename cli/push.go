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
	verbose  bool
	registry string
}

func (cmd *pushcmd) Registry() string {
	return cmd.registry
}

func (p *pushcmd) flags() []cli.Flag {
	return []cli.Flag{
		cli.BoolFlag{
			Name:        "v",
			Usage:       "verbose mode",
			Destination: &p.verbose,
		},
		cli.StringFlag{
			Name:        "registry",
			Usage:       "Sets the Docker owner for images and optionally the registry. This will be prefixed to your function name for pushing to Docker registries. eg: `--registry username` will set your Docker Hub owner. `--registry registry.hub.docker.com/username` will set the registry and owner.",
			Destination: &p.registry,
		},
	}
}

// push will take the found function and check for the presence of a
// Dockerfile, and run a three step process: parse functions file,
// push the container, and finally it will update function's route. Optionally,
// the route can be overriden inside the functions file.
func (p *pushcmd) push(c *cli.Context) error {
	setRegistryEnv(p)

	ff, err := loadFuncfile()
	if err != nil {
		if _, ok := err.(*notFoundError); ok {
			return errors.New("error: image name is missing or no function file found")
		}
		return err
	}

	fmt.Println("pushing", ff.ImageName())

	if err := dockerpush(ff); err != nil {
		return err
	}

	fmt.Printf("Function %v pushed successfully to Docker Hub.\n", ff.ImageName())
	return nil
}
