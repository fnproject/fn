package main

import (
	"github.com/iron-io/functions_go"
	"github.com/urfave/cli"
)

func run() cli.Command {
	r := routesCmd{RoutesApi: functions.NewRoutesApi()}

	return cli.Command{
		Name:      "run",
		Usage:     "run function",
		ArgsUsage: "fnclt run appName /path",
		Flags:     append(confFlags(&r.Configuration), []cli.Flag{}...),
		Action:    r.run,
	}
}
