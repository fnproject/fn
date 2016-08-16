package runner

import (
	"bytes"
	"fmt"
	"time"

	"golang.org/x/net/context"

	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/titan/common"
	"github.com/iron-io/titan/runner/agent"
	"github.com/iron-io/titan/runner/drivers"
	driverscommon "github.com/iron-io/titan/runner/drivers"
	"github.com/iron-io/titan/runner/drivers/docker"
	"github.com/iron-io/titan/runner/drivers/mock"
)

type Config struct {
	ID         string
	Ctx        context.Context
	Route      *models.Route
	Payload    string
	Timeout    time.Duration
	RequestURL string
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

	// TODO: Is this really required for Titan's driver?
	// Can we remove it?
	env := common.NewEnvironment(func(e *common.Environment) {})

	// TODO: Create a drivers.New(runnerConfig) in Titan
	driver, err := selectDriver("docker", env, &driverscommon.Config{})
	if err != nil {
		return err
	}

	ctask := &containerTask{
		cfg:    r.cfg,
		auth:   &agent.ConfigAuth{},
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

func selectDriver(driver string, env *common.Environment, conf *driverscommon.Config) (drivers.Driver, error) {
	switch driver {
	case "docker":
		docker := docker.NewDocker(env, *conf)
		return docker, nil
	case "mock":
		return mock.New(), nil
	}
	return nil, fmt.Errorf("driver %v not found", driver)
}
