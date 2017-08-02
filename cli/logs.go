package main

import (
	"context"
	"fmt"

	client "github.com/fnproject/fn/cli/client"
	fnclient "github.com/funcy/functions_go/client"
	apicall "github.com/funcy/functions_go/client/operations"
	"github.com/urfave/cli"
)

type logsCmd struct {
	client *fnclient.Functions
}

func logs() cli.Command {
	c := logsCmd{client: client.APIClient()}

	return cli.Command{
		Name:  "logs",
		Usage: "manage function calls for apps",
		Subcommands: []cli.Command{
			{
				Name:      "get",
				Aliases:   []string{"g"},
				Usage:     "get function call log info per app",
				ArgsUsage: "<app> <call-id>",
				Action:    c.get,
			},
		},
	}
}

func (log *logsCmd) get(ctx *cli.Context) error {
	app, callID := ctx.Args().Get(0), ctx.Args().Get(1)
	params := apicall.GetAppsAppCallsCallLogParams{
		Call:    callID,
		App:     app,
		Context: context.Background(),
	}
	resp, err := log.client.Operations.GetAppsAppCallsCallLog(&params)
	if err != nil {
		switch err.(type) {
		case *apicall.GetAppsAppCallsCallLogNotFound:
			return fmt.Errorf("error: %v", err.(*apicall.GetAppsAppCallsCallLogNotFound).Payload.Error.Message)
		}
		return fmt.Errorf("unexpected error: %v", err)

	}
	fmt.Print(resp.Payload.Log.Log)
	return nil
}
