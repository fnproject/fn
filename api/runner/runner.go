package runner

import (
	"fmt"
	"io"
	"time"

	"golang.org/x/net/context"

	"github.com/iron-io/titan/common"
	"github.com/iron-io/titan/runner/agent"
	"github.com/iron-io/titan/runner/drivers"
	driverscommon "github.com/iron-io/titan/runner/drivers"
	"github.com/iron-io/titan/runner/drivers/docker"
	"github.com/iron-io/titan/runner/drivers/mock"
)

type Config struct {
	ID      string
	Image   string
	Timeout time.Duration
	AppName string
	Env     map[string]string
	Stdout  io.Writer
	Stderr  io.Writer
}

type Runner struct {
	driver drivers.Driver
}

func New() (*Runner, error) {
	// TODO: Is this really required for Titan's driver?
	// Can we remove it?
	env := common.NewEnvironment(func(e *common.Environment) {})

	// TODO: Create a drivers.New(runnerConfig) in Titan
	driver, err := selectDriver("docker", env, &driverscommon.Config{})
	if err != nil {
		return nil, err
	}

	return &Runner{
		driver: driver,
	}, nil
}

func (r *Runner) Run(ctx context.Context, cfg *Config) (drivers.RunResult, error) {
	var err error

	ctask := &containerTask{
		cfg:  cfg,
		auth: &agent.ConfigAuth{},
	}

	closer, err := r.driver.Prepare(ctx, ctask)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	result, err := r.driver.Run(ctx, ctask)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (r Runner) EnsureImageExists(ctx context.Context, cfg *Config) error {
	ctask := &containerTask{
		cfg:  cfg,
		auth: &agent.ConfigAuth{},
	}

	err := r.driver.EnsureImageExists(ctx, ctask)
	if err != nil {
		return err
	}
	return nil
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
