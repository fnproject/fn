package main

import (
	"github.com/funcy/functions_go"
	"github.com/urfave/cli"
)

type imagesCmd struct {
	*functions.AppsApi
}

func images() cli.Command {
	return cli.Command{
		Name:  "images",
		Usage: "manage function images",
		Subcommands: []cli.Command{
			build(),
			deploy(),
			bump(),
			call(),
			push(),
			run(),
			testfn(),
		},
	}
}
