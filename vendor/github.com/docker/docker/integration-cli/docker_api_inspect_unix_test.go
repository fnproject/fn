// +build !windows

package main

import (
	"encoding/json"

	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/integration-cli/request"
	"github.com/go-check/check"
	"golang.org/x/net/context"
)

// #16665
func (s *DockerSuite) TestInspectAPICpusetInConfigPre120(c *check.C) {
	testRequires(c, DaemonIsLinux)
	testRequires(c, cgroupCpuset)

	name := "cpusetinconfig-pre120"
	dockerCmd(c, "run", "--name", name, "--cpuset-cpus", "0", "busybox", "true")
	cli, err := request.NewEnvClientWithVersion("v1.19")
	c.Assert(err, checker.IsNil)
	defer cli.Close()
	_, body, err := cli.ContainerInspectWithRaw(context.Background(), name, false)
	c.Assert(err, check.IsNil)

	var inspectJSON map[string]interface{}
	err = json.Unmarshal(body, &inspectJSON)
	c.Assert(err, checker.IsNil, check.Commentf("unable to unmarshal body for version 1.19"))

	config, ok := inspectJSON["Config"]
	c.Assert(ok, checker.True, check.Commentf("Unable to find 'Config'"))
	cfg := config.(map[string]interface{})
	_, ok = cfg["Cpuset"]
	c.Assert(ok, checker.True, check.Commentf("API version 1.19 expected to include Cpuset in 'Config'"))
}
