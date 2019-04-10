package docker

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/fnproject/fn/api/models"

	"github.com/fnproject/fn/api/agent/drivers"
	"github.com/fnproject/fn/api/agent/drivers/stats"
	docker "github.com/fsouza/go-dockerclient"

	"github.com/sirupsen/logrus"
)

type taskDockerTest struct {
	id         string
	cmd        string
	disableNet bool
	input      io.Reader
	output     io.Writer
	errors     io.Writer
	logURL     string
}

func (f *taskDockerTest) Command() string                                            { return f.cmd }
func (f *taskDockerTest) EnvVars() map[string]string                                 { return map[string]string{} }
func (f *taskDockerTest) Id() string                                                 { return f.id }
func (f *taskDockerTest) Group() string                                              { return "" }
func (f *taskDockerTest) Image() string                                              { return "busybox" }
func (f *taskDockerTest) Logger() (stdout, stderr io.Writer)                         { return f.output, f.errors }
func (f *taskDockerTest) WriteStat(context.Context, stats.Stat)                      { /* TODO */ }
func (f *taskDockerTest) Volumes() [][2]string                                       { return [][2]string{} }
func (f *taskDockerTest) Memory() uint64                                             { return 256 * 1024 * 1024 }
func (f *taskDockerTest) CPUs() uint64                                               { return 0 }
func (f *taskDockerTest) FsSize() uint64                                             { return 0 }
func (f *taskDockerTest) TmpFsSize() uint64                                          { return 0 }
func (f *taskDockerTest) WorkDir() string                                            { return "" }
func (f *taskDockerTest) Close()                                                     {}
func (f *taskDockerTest) WrapClose(func(func()) func())                              {}
func (f *taskDockerTest) WrapBeforeCall(func(drivers.BeforeCall) drivers.BeforeCall) {}
func (f *taskDockerTest) WrapAfterCall(func(drivers.AfterCall) drivers.AfterCall)    {}
func (f *taskDockerTest) Input() io.Reader                                           { return f.input }
func (f *taskDockerTest) Extensions() map[string]string                              { return nil }
func (f *taskDockerTest) LoggerConfig() drivers.LoggerConfig {
	return drivers.LoggerConfig{URL: f.logURL}
}
func (f *taskDockerTest) UDSAgentPath() string  { return "" }
func (f *taskDockerTest) UDSDockerPath() string { return "" }
func (f *taskDockerTest) UDSDockerDest() string { return "" }
func (f *taskDockerTest) DisableNet() bool      { return f.disableNet }

func (f *taskDockerTest) BeforeCall(context.Context, *models.Call, drivers.CallExtensions) error {
	return nil
}
func (f *taskDockerTest) AfterCall(context.Context, *models.Call, drivers.CallExtensions) error {
	return nil
}

func createTask(id string) *taskDockerTest {
	return &taskDockerTest{
		id: id,
	}
}

func commonCookiePull(ctx context.Context, cookie drivers.Cookie) error {
	shouldPull, err := cookie.ValidateImage(ctx)
	if err != nil {
		return err
	}
	if shouldPull {
		err = cookie.PullImage(ctx)
		if err != nil {
			return err
		}
		shouldPull, err = cookie.ValidateImage(ctx)
		if err != nil || shouldPull {
			return err
		}
	}
	return nil
}

func commonCookieRun(ctx context.Context, cookie drivers.Cookie) (error, drivers.RunResult) {
	err := cookie.CreateContainer(ctx)
	if err != nil {
		return err, nil
	}

	waiter, err := cookie.Run(ctx)
	if err != nil {
		return err, nil
	}
	return nil, waiter.Wait(ctx)
}

type taskDockerVolumeTest struct {
	taskDockerTest
}

func (f *taskDockerVolumeTest) Image() string { return "fnproject/fn-test-volume:latest" }

func TestVolumeValidation(t *testing.T) {
	dkr := NewDocker(drivers.Config{})
	defer dkr.Close()

	ctx := context.Background()
	var output bytes.Buffer
	var errors bytes.Buffer

	task := &taskDockerVolumeTest{taskDockerTest{id: "test-docker"}}
	task.output = &output
	task.errors = &errors

	cookie, err := dkr.CreateCookie(ctx, task)
	if err != nil {
		t.Fatal("Couldn't create task cookie")
	}

	defer cookie.Close(ctx)

	shouldPull, err := cookie.ValidateImage(ctx)

	if shouldPull == true {
		t.Fatalf("shouldPull must be false, because the image [%s] must be locally available", task.Image())
	}

	if err != ErrImageWithVolume {
		t.Fatalf("ValidateImage must fail with error: %s", ErrImageWithVolume)
	}

}

func TestRunnerDocker(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(30)*time.Second)
	defer cancel()

	dkr := NewDocker(drivers.Config{})
	defer dkr.Close()

	var output bytes.Buffer
	var errors bytes.Buffer

	task := createTask("test-docker")
	task.output = &output
	task.errors = &errors

	cookie, err := dkr.CreateCookie(ctx, task)
	if err != nil {
		t.Fatal("Couldn't create task cookie")
	}

	defer cookie.Close(ctx)

	err = commonCookiePull(ctx, cookie)
	if err != nil {
		t.Fatal(err)
	}
	err, result := commonCookieRun(ctx, cookie)
	if err != nil {
		t.Fatal(err)
	}
	if result.Error() != nil {
		t.Fatal(result.Error())
	}

	if result.Status() != "success" {
		t.Fatalf("Test should successfully run the image: %s output: %s errors: %s",
			result.Error(), output.String(), errors.String())
	}
}

func TestRunnerDockerNetworks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(30)*time.Second)
	defer cancel()

	dkr := NewDocker(drivers.Config{
		DockerNetworks: "test1 test2",
	})
	defer dkr.Close()

	task1 := createTask("test-docker1")
	task2 := createTask("test-docker2")

	cookie1, err := dkr.CreateCookie(ctx, task1)
	if err != nil {
		t.Fatal("Couldn't create task1 cookie")
	}

	defer cookie1.Close(ctx)

	cookie2, err := dkr.CreateCookie(ctx, task2)
	if err != nil {
		t.Fatal("Couldn't create task2 cookie")
	}

	defer cookie2.Close(ctx)

	c1 := cookie1.(*cookie)
	c2 := cookie2.(*cookie)

	var tally = map[string]uint64{
		"test1": 0,
		"test2": 0,
	}

	tally[c1.netId]++
	tally[c2.netId]++

	for key, val := range tally {
		if val != 1 {
			t.Fatalf("netId unbalanced network usage for %s expected 1 got %d", key, val)
		}
	}
}

func TestRunnerDockerVersion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(6)*time.Second)
	defer cancel()

	dkr := NewDocker(drivers.Config{})
	if dkr == nil {
		t.Fatal("should not be nil")
	}
	defer dkr.Close()

	dkr.conf.ServerVersion = "1.0.0"
	err := checkDockerVersion(ctx, dkr)
	if err != nil {
		t.Fatal(err)
	}

	dkr.conf.ServerVersion = "9999.0.0"
	err = checkDockerVersion(ctx, dkr)
	if err == nil {
		t.Fatal("should have failed")
	}
}

func TestRunnerDockerStdout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(30)*time.Second)
	defer cancel()

	dkr := NewDocker(drivers.Config{})
	defer dkr.Close()

	var output bytes.Buffer
	var errors bytes.Buffer

	task := createTask("test-docker-stdin")
	task.output = &output
	task.errors = &errors
	task.cmd = "echo hello"

	cookie, err := dkr.CreateCookie(ctx, task)
	if err != nil {
		t.Fatal("Couldn't create task cookie")
	}
	defer cookie.Close(ctx)

	err = commonCookiePull(ctx, cookie)
	if err != nil {
		t.Fatal(err)
	}
	err, result := commonCookieRun(ctx, cookie)
	if err != nil {
		t.Fatal(err)
	}
	if result.Error() != nil {
		t.Fatal(result.Error())
	}

	if result.Status() != "success" {
		t.Fatalf("Test should successfully run the image: %s output: %s errors: %s",
			result.Error(), output.String(), errors.String())
	}

	expect := "hello"
	got := output.String()
	if !strings.Contains(got, expect) {
		t.Errorf("Test expected output to contain '%s', got '%s'", expect, got)
	}
}

//
//func TestRegistry(t *testing.T) {
//	image := "fnproject/fn-test-utils"
//
//	sizer, err := CheckRegistry(context.Background(), image, docker.AuthConfiguration{})
//	if err != nil {
//		t.Fatal("expected registry check not to fail, got:", err)
//	}
//
//	size, err := sizer.Size()
//	if err != nil {
//		t.Fatal("expected sizer not to fail, got:", err)
//	}
//
//	if size <= 0 {
//		t.Fatal("expected positive size for image that exists, got size:", size)
//	}
//}

func newTestClient(ctx context.Context) *docker.Client {
	client, err := docker.NewClientFromEnv()
	if err != nil {
		logrus.WithError(err).Fatal("couldn't create docker client")
	}

	if err := client.Ping(); err != nil {
		logrus.WithError(err).Fatal("couldn't connect to docker daemon")
	}

	return client
}

func createContainer(ctx context.Context, client *docker.Client, id, labelTag, labelId string) error {
	opts := docker.CreateContainerOptions{
		Name: id,
		Config: &docker.Config{
			Cmd:          strings.Fields("tail -f /dev/null"),
			Hostname:     id,
			Image:        "busybox",
			Volumes:      map[string]struct{}{},
			OpenStdin:    false,
			AttachStdout: false,
			AttachStderr: false,
			AttachStdin:  false,
			StdinOnce:    false,
			Labels: map[string]string{
				FnAgentClassifierLabel: labelTag,
				FnAgentInstanceLabel:   labelId,
			},
		},
		HostConfig: &docker.HostConfig{
			LogConfig: docker.LogConfig{
				Type: "none",
			},
		},
		Context: ctx,
	}

	_, err := client.CreateContainer(opts)
	if err != nil {
		return err
	}

	return client.StartContainerWithContext(id, nil, ctx)
}

func destroyContainer(client *docker.Client, id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(5)*time.Second)
	defer cancel()
	err := client.RemoveContainer(docker.RemoveContainerOptions{
		ID:            id,
		Force:         true,
		RemoveVolumes: true,
		Context:       ctx,
	})
	return err
}

// case1 - running container that does not belong us (tag mismatch) (should not get killed)
func TestRunnerDockerCleanCase1(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(30)*time.Second)
	defer cancel()
	client := newTestClient(ctx)

	// spin up a container
	id := "TestRunnerDockerCleanCase1"
	err := createContainer(ctx, client, id, "fn-agent-case-unknown", "fn-agent-instance-unknown")
	defer destroyContainer(client, id)
	if err != nil {
		t.Fatal(err)
	}

	// spin up docker driver
	dkr := NewDocker(drivers.Config{ContainerLabelTag: "fn-agent-case-1"})
	defer dkr.Close()

	// our container should not get killed (let's wait for 5 secs)
	ctx2, cancel2 := context.WithTimeout(ctx, time.Duration(5)*time.Second)
	defer cancel2()

	exitCode, err := client.WaitContainerWithContext(id, ctx2)
	if exitCode != 0 || err != context.DeadlineExceeded {
		t.Fatalf("err=%v exit=%d", err, exitCode)
	}
}

// case2 - running container is ours (should not get killed)
func TestRunnerDockerCleanCase2(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(30)*time.Second)
	defer cancel()
	client := newTestClient(ctx)

	// spin up a container
	id := "TestRunnerDockerCleanCase2"
	err := createContainer(ctx, client, id, "fn-agent-case-2", "fn-agent-instance-2")
	defer destroyContainer(client, id)
	if err != nil {
		t.Fatal(err)
	}

	// spin up docker driver
	dkr := NewDocker(drivers.Config{ContainerLabelTag: "fn-agent-case-2", InstanceId: "fn-agent-instance-2"})
	defer dkr.Close()

	// our container should not get killed (let's wait for 5 secs)
	ctx2, cancel2 := context.WithTimeout(ctx, time.Duration(5)*time.Second)
	defer cancel2()

	exitCode, err := client.WaitContainerWithContext(id, ctx2)
	if exitCode != 0 || err != context.DeadlineExceeded {
		t.Fatalf("err=%v exit=%d", err, exitCode)
	}
}

// case3 - running container that does not belong us (should get destroyed)
func TestRunnerDockerCleanCase3(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(30)*time.Second)
	defer cancel()
	client := newTestClient(ctx)

	// spin up a container
	id := "TestRunnerDockerCleanCase3"
	err := createContainer(ctx, client, id, "fn-agent-case-3", "fn-agent-instance-unknown")
	defer destroyContainer(client, id)
	if err != nil {
		t.Fatal(err)
	}

	// spin up docker driver
	dkr := NewDocker(drivers.Config{ContainerLabelTag: "fn-agent-case-3", InstanceId: "fn-agent-instance-3"})
	defer dkr.Close()

	// our container should get killed (let's wait for 5 secs)
	ctx2, cancel2 := context.WithTimeout(ctx, time.Duration(5)*time.Second)
	defer cancel2()

	exitCode, err := client.WaitContainerWithContext(id, ctx2)
	if exitCode != 137 || err != nil {
		t.Fatalf("err=%v exit=%d", err, exitCode)
	}
}

// case4 - dead container that does not belong us (should get destroyed)
func TestRunnerDockerCleanCase4(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(30)*time.Second)
	defer cancel()
	client := newTestClient(ctx)

	// spin up a container
	id := "TestRunnerDockerCleanCase4"
	err := createContainer(ctx, client, id, "fn-agent-case-4", "fn-agent-instance-unknown")
	defer destroyContainer(client, id)
	if err != nil {
		t.Fatal(err)
	}

	// stop container
	err = client.KillContainer(docker.KillContainerOptions{
		ID:      id,
		Context: ctx,
	})
	if err != nil {
		t.Fatal(err)
	}

	// spin up docker driver
	dkr := NewDocker(drivers.Config{ContainerLabelTag: "fn-agent-case-4", InstanceId: "fn-agent-instance-4"})
	defer dkr.Close()

	exitCode, err := client.WaitContainerWithContext(id, ctx)
	if exitCode != 137 || err != nil {
		t.Fatalf("err=%v exit=%d", err, exitCode)
	}

	// let's wait for 5 secs, then destroy should return error.
	select {
	case <-ctx.Done():
		t.Fatal("timeout")
	case <-time.After(5 * time.Second):
	}

	// This should fail with NoSuchContainer
	err = client.RemoveContainer(docker.RemoveContainerOptions{
		ID:            id,
		Force:         true,
		RemoveVolumes: true,
		Context:       ctx,
	})
	_, containerNotFound := err.(*docker.NoSuchContainer)
	if !containerNotFound {
		t.Fatalf("Expected container not found, but got %v", err)
	}
}

// create container with disable net.
// query container HostConfig, where NetworkMode should be 'none'
func TestRunnerDockerNoNetwork(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(30)*time.Second)
	defer cancel()

	dkr := NewDocker(drivers.Config{})
	defer dkr.Close()

	task := createTask("test-docker-no-net")
	task.disableNet = true

	cookie, err := dkr.CreateCookie(ctx, task)
	if err != nil {
		t.Fatal("Couldn't create task cookie")
	}

	defer cookie.Close(ctx)

	err = commonCookiePull(ctx, cookie)
	if err != nil {
		t.Fatal(err)
	}

	err = cookie.CreateContainer(ctx)
	if err != nil {
		t.Fatal("Couldn't create container test")
	}

	client := newTestClient(ctx)

	c, err := client.InspectContainerWithContext(task.Id(), ctx)
	if err != nil {
		t.Fatalf("Couldn't inspect container test %v", err)
	}
	if c == nil || c.HostConfig == nil || c.HostConfig.NetworkMode != "none" {
		t.Fatalf("Couldn't create none network container: %+v", c)
	}

	// We could make busybox execute a 'ip link' or 'ip address' and parse the output, but
	// this is unnecessary as NetworkMode=none is well known. (eg. docker run --network none)
	// https://docs.docker.com/network/none/
}

func TestRunnerDockerInvalidSyslog(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(30)*time.Second)
	defer cancel()

	dkr := NewDocker(drivers.Config{})
	defer dkr.Close()

	task := createTask("test-docker-no-net")
	task.logURL = "tcp://invalid:9999"

	cookie, err := dkr.CreateCookie(ctx, task)
	if err != nil {
		t.Fatal("Couldn't create task cookie")
	}
	defer cookie.Close(ctx)

	err = commonCookiePull(ctx, cookie)
	if err != nil {
		t.Fatal(err)
	}
	err, _ = commonCookieRun(ctx, cookie)
	if err == nil {
		t.Fatal("Error expected when running with invalid syslog configuration")
	}
	if err.Error() != "Syslog Unavailable" {
		t.Fatalf("Error message expected: `Syslog Unavailable`, got `%s`", err)
	}

}
