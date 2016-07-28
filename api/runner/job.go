package runner

import (
	"io"
	"os"

	dockercli "github.com/fsouza/go-dockerclient"
	"github.com/iron-io/titan/runner/tasker"
	titan_models "github.com/iron-io/titan/runner/tasker/client/models"
)

type WrapperJob struct {
	auth tasker.Auther
	log  *os.File
	m    *titan_models.Job
}

func (f *WrapperJob) Command() string { return "" }

func (f *WrapperJob) EnvVars() map[string]string {
	m := map[string]string{
		"JOB_ID":  f.Id(),
		"PAYLOAD": f.m.Payload,
	}
	for k, v := range f.m.EnvVars {
		m[k] = v
	}
	return m
}

func (f *WrapperJob) Id() string                         { return f.m.ID }
func (f *WrapperJob) Group() string                      { return f.m.GroupName }
func (f *WrapperJob) Image() string                      { return *f.m.Image }
func (f *WrapperJob) Timeout() uint                      { return uint(*f.m.Timeout) }
func (f *WrapperJob) Logger() (stdout, stderr io.Writer) { return f.log, f.log }
func (f *WrapperJob) Volumes() [][2]string               { return [][2]string{} }
func (f *WrapperJob) WorkDir() string                    { return "" }

func (f *WrapperJob) Close() {}

func (f *WrapperJob) DockerAuth() []dockercli.AuthConfiguration {
	return f.auth.Auth(f.Image())
}
