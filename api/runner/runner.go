package runner

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"golang.org/x/net/context"

	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/titan/common"
	"github.com/iron-io/titan/runner/agent"
	"github.com/iron-io/titan/runner/configloader"
	"github.com/iron-io/titan/runner/drivers"
	"github.com/iron-io/titan/runner/drivers/docker"
	"github.com/iron-io/titan/runner/drivers/mock"
	titan_models "github.com/iron-io/titan/runner/tasker/client/models"
)

type Config struct {
	Route    *models.Route
	Endpoint string
	Payload  string
	Timeout  time.Duration
}

type Runner struct {
	cfg    *Config
	status string
	result []byte
}

func New(cfg *Config) *Runner {
	return &Runner{
		cfg: cfg,
	}
}

func (r *Runner) Start() error {
	image := r.cfg.Route.Image
	payload := r.cfg.Payload
	timeout := int32(r.cfg.Timeout.Seconds())

	var err error

	runnerConfig := configloader.RunnerConfiguration()

	au := agent.ConfigAuth{runnerConfig.Registries}
	env := common.NewEnvironment(func(e *common.Environment) {})
	driver, err := selectDriver(env, runnerConfig)
	if err != nil {
		return err
	}

	job := &titan_models.Job{
		NewJob: titan_models.NewJob{
			Image:   &image,
			Payload: payload,
			Timeout: &timeout,
		},
	}

	tempLog, err := ensureLogFile(job)
	if err != nil {
		return err
	}
	defer tempLog.Close()

	wjob := &WrapperJob{
		auth: &au,
		m:    job,
		log:  tempLog,
	}

	result, err := driver.Run(context.Background(), wjob)
	if err != nil {
		return err
	}

	b, _ := ioutil.ReadFile(tempLog.Name())
	r.result = b
	r.status = result.Status()

	return nil
}

func (r Runner) Result() []byte {
	return r.result
}

func (r Runner) Status() string {
	return r.status
}

func ensureLogFile(job *titan_models.Job) (*os.File, error) {
	log, err := ioutil.TempFile("", fmt.Sprintf("titan-log-%s", job.ID))
	if err != nil {
		return nil, fmt.Errorf("couldn't open task log for writing: %v", err)
	}
	return log, nil
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
