package main

import (
	"context"
	"fmt"

	fnclient "github.com/funcy/functions_go/client"
	apicall "github.com/funcy/functions_go/client/call"
	"github.com/funcy/functions_go/models"
	"github.com/urfave/cli"
	client "gitlab-odx.oracle.com/odx/functions/fn/client"
)

type callsCmd struct {
	client *fnclient.Functions
}

func calls() cli.Command {
	c := callsCmd{client: client.APIClient()}

	return cli.Command{
		Name:  "calls",
		Usage: "manage function calls",
		Subcommands: []cli.Command{
			{
				Name:      "get",
				Aliases:   []string{"g"},
				Usage:     "get function call info",
				ArgsUsage: "<call-id>",
				Action:    c.get,
			},
			{
				Name:      "list",
				Aliases:   []string{"l"},
				Usage:     "list all calls for specific route",
				ArgsUsage: "<app> <route>",
				Action:    c.list,
			},
		},
	}
}

func printCalls(calls []*models.Call) {
	for _, call := range calls {
		fmt.Println(fmt.Sprintf(
			"ID: %v\n"+
				"App: %v\n"+
				"Route: %v\n"+
				"Created At: %v\n"+
				"Started At: %v\n"+
				"Completed At: %v\n"+
				"Status: %v\n",
			call.ID, call.AppName, call.Path, call.CreatedAt,
			call.StartedAt, call.CompletedAt, call.Status))
	}
}

func (call *callsCmd) get(ctx *cli.Context) error {
	callID := ctx.Args().Get(0)
	params := apicall.GetCallsCallParams{
		Call:    callID,
		Context: context.Background(),
	}
	resp, err := call.client.Call.GetCallsCall(&params)
	if err != nil {
		switch err.(type) {
		case *apicall.GetCallsCallNotFound:
			return fmt.Errorf("error: %v", err.(*apicall.GetCallsCallNotFound).Payload.Error.Message)
		}
		return fmt.Errorf("unexpected error: %v", err)

	}
	printCalls([]*models.Call{resp.Payload.Call})
	return nil
}

func (call *callsCmd) list(ctx *cli.Context) error {
	app, route := ctx.Args().Get(0), ctx.Args().Get(1)
	params := apicall.GetAppsAppCallsRouteParams{
		App:     app,
		Route:   route,
		Context: context.Background(),
	}
	resp, err := call.client.Call.GetAppsAppCallsRoute(&params)
	if err != nil {
		switch err.(type) {
		case *apicall.GetCallsCallNotFound:
			return fmt.Errorf("error: %v", err.(*apicall.GetCallsCallNotFound).Payload.Error.Message)
		}
		return fmt.Errorf("unexpected error: %v", err)

	}
	printCalls(resp.Payload.Calls)
	return nil
}
