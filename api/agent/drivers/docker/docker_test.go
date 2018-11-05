package docker

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/fnproject/fn/api/agent/drivers"
)

type taskDockerTest struct {
	id     string
	input  io.Reader
	output io.Writer
	errors io.Writer
}

func (f *taskDockerTest) Command() string                         { return "" }
func (f *taskDockerTest) EnvVars() map[string]string              { return map[string]string{} }
func (f *taskDockerTest) Id() string                              { return f.id }
func (f *taskDockerTest) Group() string                           { return "" }
func (f *taskDockerTest) Image() string                           { return "hello-world" }
func (f *taskDockerTest) Timeout() time.Duration                  { return 30 * time.Second }
func (f *taskDockerTest) Logger() (stdout, stderr io.Writer)      { return f.output, f.errors }
func (f *taskDockerTest) WriteStat(context.Context, drivers.Stat) { /* TODO */ }
func (f *taskDockerTest) Volumes() [][2]string                    { return [][2]string{} }
func (f *taskDockerTest) Memory() uint64                          { return 256 * 1024 * 1024 }
func (f *taskDockerTest) CPUs() uint64                            { return 0 }
func (f *taskDockerTest) FsSize() uint64                          { return 0 }
func (f *taskDockerTest) TmpFsSize() uint64                       { return 0 }
func (f *taskDockerTest) WorkDir() string                         { return "" }
func (f *taskDockerTest) Close()                                  {}
func (f *taskDockerTest) Input() io.Reader                        { return f.input }
func (f *taskDockerTest) Extensions() map[string]string           { return nil }
func (f *taskDockerTest) LoggerConfig() drivers.LoggerConfig      { return drivers.LoggerConfig{} }
func (f *taskDockerTest) UDSAgentPath() string                    { return "" }
func (f *taskDockerTest) UDSDockerPath() string                   { return "" }
func (f *taskDockerTest) UDSDockerDest() string                   { return "" }

func TestRunnerDocker(t *testing.T) {
	dkr := NewDocker(drivers.Config{})
	ctx := context.Background()
	var output bytes.Buffer
	var errors bytes.Buffer

	task := &taskDockerTest{"test-docker", bytes.NewBufferString(`{"isDebug": true}`), &output, &errors}

	cookie, err := dkr.CreateCookie(ctx, task)
	if err != nil {
		t.Fatal("Couldn't create task cookie")
	}

	defer cookie.Close(ctx)

	err = dkr.PrepareCookie(ctx, cookie)
	if err != nil {
		t.Fatal("Couldn't prepare task test")
	}

	waiter, err := cookie.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	result := waiter.Wait(ctx)
	if result.Error() != nil {
		t.Fatal(result.Error())
	}

	if result.Status() != "success" {
		t.Fatalf("Test should successfully run the image: %s output: %s errors: %s",
			result.Error(), output.String(), errors.String())
	}
}

func TestRunnerDockerNetworks(t *testing.T) {
	dkr := NewDocker(drivers.Config{
		DockerNetworks: "test1 test2",
	})

	ctx := context.Background()
	var output bytes.Buffer
	var errors bytes.Buffer

	task1 := &taskDockerTest{"test-docker1", bytes.NewBufferString(`{"isDebug": true}`), &output, &errors}
	task2 := &taskDockerTest{"test-docker2", bytes.NewBufferString(`{"isDebug": true}`), &output, &errors}

	cookie1, err := dkr.CreateCookie(ctx, task1)
	if err != nil {
		t.Fatal("Couldn't create task1 cookie")
	}

	defer cookie1.Close(ctx)

	err = dkr.PrepareCookie(ctx, cookie1)
	if err != nil {
		t.Fatal("Couldn't prepare task1 test")
	}

	cookie2, err := dkr.CreateCookie(ctx, task2)
	if err != nil {
		t.Fatal("Couldn't create task2 cookie")
	}

	defer cookie2.Close(ctx)

	err = dkr.PrepareCookie(ctx, cookie2)
	if err != nil {
		t.Fatal("Couldn't prepare task2 test")
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

	dkr := NewDocker(drivers.Config{
		ServerVersion: "0.0.0",
	})
	if dkr == nil {
		t.Fatal("should not be nil")
	}

	err := checkDockerVersion(dkr, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	err = checkDockerVersion(dkr, "9999.0.0")
	if err == nil {
		t.Fatal("should have failed")
	}
}

func TestRunnerDockerStdout(t *testing.T) {
	dkr := NewDocker(drivers.Config{})
	ctx := context.Background()

	var output bytes.Buffer
	var errors bytes.Buffer

	task := &taskDockerTest{"test-docker-stdin", bytes.NewBufferString(""), &output, &errors}

	cookie, err := dkr.CreateCookie(ctx, task)
	if err != nil {
		t.Fatal("Couldn't create task cookie")
	}

	defer cookie.Close(ctx)

	err = dkr.PrepareCookie(ctx, cookie)
	if err != nil {
		t.Fatal("Couldn't prepare task test")
	}

	waiter, err := cookie.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	result := waiter.Wait(ctx)
	if result.Error() != nil {
		t.Fatal(result.Error())
	}

	if result.Status() != "success" {
		t.Fatalf("Test should successfully run the image: %s output: %s errors: %s",
			result.Error(), output.String(), errors.String())
	}

	// if hello world image changes, change dis
	expect := "Hello from Docker!"
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
