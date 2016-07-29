package runner

import (
	"io"

	dockercli "github.com/fsouza/go-dockerclient"
	"github.com/iron-io/titan/runner/tasker"
)

type containerTask struct {
	auth   tasker.Auther
	stdout io.Writer
	stderr io.Writer
	cfg    *Config
}

func (t *containerTask) Command() string { return "" }

func (t *containerTask) EnvVars() map[string]string {
	env := map[string]string{
		"PAYLOAD": t.cfg.Payload,
	}
	return env
}

func (t *containerTask) Id() string                         { return "" }
func (t *containerTask) Group() string                      { return "" }
func (t *containerTask) Image() string                      { return t.cfg.Route.Image }
func (t *containerTask) Timeout() uint                      { return uint(t.cfg.Timeout.Seconds()) }
func (t *containerTask) Logger() (stdout, stderr io.Writer) { return t.stdout, t.stderr }
func (t *containerTask) Volumes() [][2]string               { return [][2]string{} }
func (t *containerTask) WorkDir() string                    { return "" }

func (t *containerTask) Close() {}

func (t *containerTask) DockerAuth() []dockercli.AuthConfiguration {
	return t.auth.Auth(t.Image())
}
