package runner

import (
	"io"

	dockercli "github.com/fsouza/go-dockerclient"
	"github.com/iron-io/titan/runner/tasker"
	"github.com/iron-io/titan/runner/drivers"
)

type containerTask struct {
	auth tasker.Auther
	cfg  *Config
}

func (t *containerTask) Command() string { return "" }

func (t *containerTask) EnvVars() map[string]string {
	env := map[string]string{
		"PAYLOAD":     t.cfg.Payload,
		"REQUEST_URL": t.cfg.RequestURL,
	}
	return env
}

func (t *containerTask) Labels() map[string]string {
	return map[string]string{
		"LogName": t.cfg.AppName,
	}
}

func (t *containerTask) Id() string                         { return t.cfg.ID }
func (t *containerTask) Group() string                      { return "" }
func (t *containerTask) Image() string                      { return t.cfg.Route.Image }
func (t *containerTask) Timeout() uint                      { return uint(t.cfg.Timeout.Seconds()) }
func (t *containerTask) Logger() (stdout, stderr io.Writer) { return t.cfg.Stdout, t.cfg.Stderr }
func (t *containerTask) Volumes() [][2]string               { return [][2]string{} }
func (t *containerTask) WorkDir() string                    { return "" }

func (t *containerTask) Close() {}
func (t *containerTask) WriteStat(drivers.Stat) {} 

func (t *containerTask) DockerAuth() []dockercli.AuthConfiguration {
	return t.auth.Auth(t.Image())
}
