package runner

import (
	"bytes"
	"fmt"
	"time"

	"golang.org/x/net/context"

	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/titan/common"
	"github.com/iron-io/titan/runner/agent"
	"github.com/iron-io/titan/runner/configloader"
	"github.com/iron-io/titan/runner/drivers"
	"github.com/iron-io/titan/runner/drivers/docker"
	"github.com/iron-io/titan/runner/drivers/mock"
)

type Config struct {
	Ctx      context.Context
	Route    *models.Route
	Endpoint string
	Payload  string
	Timeout  time.Duration
}

type Runner struct {
	cfg    *Config
	status string
	out    bytes.Buffer
	err    bytes.Buffer
}

func New(cfg *Config) *Runner {
	return &Runner{
		cfg: cfg,
	}
}

func (r *Runner) Run() error {
	var err error

	runnerConfig := configloader.RunnerConfiguration()

	au := agent.ConfigAuth{runnerConfig.Registries}

	// TODO: Is this really required for Titan's driver?
	// Can we remove it?
	env := common.NewEnvironment(func(e *common.Environment) {})

	// TODO: Create a drivers.New(runnerConfig) in Titan
	driver, err := selectDriver(env, runnerConfig)
	if err != nil {
		return err
	}

	ctask := &containerTask{
		cfg:    r.cfg,
		auth:   &au,
		stdout: &r.out,
		stderr: &r.err,
	}

	result, err := driver.Run(r.cfg.Ctx, ctask)
	if err != nil {
		return err
	}

	r.status = result.Status()

	return nil
}

func (r *Runner) ReadOut() []byte {
	return r.out.Bytes()
}

func (r Runner) ReadErr() []byte {
	return r.err.Bytes()
}

func (r Runner) Status() string {
	return r.status
}

func selectDriver(env *common.Environment, conf *agent.Config) (drivers.Driver, error) {
	switch conf.Driver {
	case "docker":
		docker := docker.NewDocker(env, conf.DriverConfig)
		return docker, nil
	case "mock":
		return mock.New(), nil
	}
	return nil, fmt.Errorf("driver %v not found", conf.Driver)
}
