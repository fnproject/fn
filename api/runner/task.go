package runner

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/docker/cli/cli/config/configfile"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/runner/drivers"
	"github.com/fnproject/fn/api/runner/protocol"
	docker "github.com/fsouza/go-dockerclient"
)

var registries dockerRegistries

func init() {
	// TODO this is docker specific. and the docker client is capable of doing this, remove & test

	// Attempt to fetch it from an environment variable
	regsettings := os.Getenv("DOCKER_AUTH")

	if regsettings == "" {
		u, err := user.Current()
		if err == nil {
			var config configfile.ConfigFile
			cfile, err := os.Open(filepath.Join(u.HomeDir, ".docker", "config.json"))
			if err != nil {
				return
			}
			err = config.LoadFromReader(cfile)
			if err != nil {
				return
			}

			var regs []dockerRegistry
			for _, auth := range config.AuthConfigs {
				regs = append(regs, dockerRegistry{
					Username: auth.Username,
					Password: auth.Password,
					Name:     auth.ServerAddress,
				})
			}

			registries = dockerRegistries(regs)
		}
	} else {
		// If we have settings, unmarshal them
		json.Unmarshal([]byte(regsettings), &registries)
	}

}

// TODO task.Config should implement the interface. this is sad :(
// implements drivers.ContainerTask
type containerTask struct {
	ctx    context.Context
	cfg    *models.Task
	canRun chan bool
}

func (t *containerTask) EnvVars() map[string]string {
	if protocol.IsStreamable(protocol.Protocol(t.cfg.Format)) {
		return t.cfg.BaseEnv
	}
	return t.cfg.EnvVars
}

func (t *containerTask) Labels() map[string]string {
	// TODO this seems inaccurate? is this used by anyone (dev or not)?
	return map[string]string{"LogName": t.cfg.AppName}
}

func (t *containerTask) Command() string                { return "" }
func (t *containerTask) Input() io.Reader               { return t.cfg.Stdin }
func (t *containerTask) Id() string                     { return t.cfg.ID }
func (t *containerTask) Image() string                  { return t.cfg.Image }
func (t *containerTask) Timeout() time.Duration         { return t.cfg.TimeoutDuration() }
func (t *containerTask) Logger() (io.Writer, io.Writer) { return t.cfg.Stdout, t.cfg.Stderr }
func (t *containerTask) Volumes() [][2]string           { return [][2]string{} }
func (t *containerTask) Memory() uint64                 { return t.cfg.Memory * 1024 * 1024 } // convert MB
func (t *containerTask) WorkDir() string                { return "" }
func (t *containerTask) Close()                         {}
func (t *containerTask) WriteStat(drivers.Stat)         {}

// Implementing the docker.AuthConfiguration interface.  Pulling in
// the docker repo password from environment variables
func (t *containerTask) DockerAuth() (docker.AuthConfiguration, error) {
	reg, _, _ := drivers.ParseImage(t.Image())
	authconfig := docker.AuthConfiguration{}

	if customAuth := registries.Find(reg); customAuth != nil {
		authconfig = docker.AuthConfiguration{
			Password:      customAuth.Password,
			ServerAddress: customAuth.Name,
			Username:      customAuth.Username,
		}
	}

	return authconfig, nil
}
