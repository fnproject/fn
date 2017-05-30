package main

import (
	"fmt"
	"net/url"
	"os"

	vers "gitlab-odx.oracle.com/odx/functions/api/version"
	functions "github.com/iron-io/functions_go"
	"github.com/urfave/cli"
)

func version() cli.Command {
	r := versionCmd{VersionApi: functions.NewVersionApi()}
	return cli.Command{
		Name:   "version",
		Usage:  "displays fn and functions daemon versions",
		Action: r.version,
	}
}

type versionCmd struct {
	*functions.VersionApi
}

func (r *versionCmd) version(c *cli.Context) error {
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}

	u, err := url.Parse(apiURL)
	if err != nil {
		return err
	}
	r.Configuration.BasePath = u.String()

	fmt.Println("Client version:", vers.Version)
	v, _, err := r.VersionGet()
	if err != nil {
		return err
	}
	fmt.Println("Server version", v.Version)
	return nil
}
