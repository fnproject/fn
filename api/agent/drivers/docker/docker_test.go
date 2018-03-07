package docker

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/fnproject/fn/api/agent/drivers"
	"github.com/fsouza/go-dockerclient"
)

type taskDockerTest struct {
	id     string
	input  io.Reader
	output io.Writer
	errors io.Writer
}

func (f *taskDockerTest) Command() string { return "" }
func (f *taskDockerTest) EnvVars() map[string]string {
	return map[string]string{"FN_FORMAT": "default"}
}
func (f *taskDockerTest) Labels() map[string]string               { return nil }
func (f *taskDockerTest) Id() string                              { return f.id }
func (f *taskDockerTest) Group() string                           { return "" }
func (f *taskDockerTest) Image() string                           { return "fnproject/fn-test-utils" }
func (f *taskDockerTest) Timeout() time.Duration                  { return 30 * time.Second }
func (f *taskDockerTest) Logger() (stdout, stderr io.Writer)      { return f.output, f.errors }
func (f *taskDockerTest) WriteStat(context.Context, drivers.Stat) { /* TODO */ }
func (f *taskDockerTest) Volumes() [][2]string                    { return [][2]string{} }
func (f *taskDockerTest) Memory() uint64                          { return 256 * 1024 * 1024 }
func (f *taskDockerTest) CPUs() uint64                            { return 0 }
func (f *taskDockerTest) WorkDir() string                         { return "" }
func (f *taskDockerTest) Close()                                  {}
func (f *taskDockerTest) Input() io.Reader                        { return f.input }

func TestRunnerDocker(t *testing.T) {
	dkr := NewDocker(drivers.Config{})
	ctx := context.Background()
	var output bytes.Buffer
	var errors bytes.Buffer

	task := &taskDockerTest{"test-docker", bytes.NewBufferString(`{"isDebug": true}`), &output, &errors}

	cookie, err := dkr.Prepare(ctx, task)
	if err != nil {
		t.Fatal("Couldn't prepare task test")
	}
	defer cookie.Close(ctx)

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

func TestRunnerDockerStdin(t *testing.T) {
	dkr := NewDocker(drivers.Config{})
	ctx := context.Background()

	input := `{"echoContent": "italian parsley", "isDebug": true}`

	var output bytes.Buffer
	var errors bytes.Buffer

	task := &taskDockerTest{"test-docker-stdin", bytes.NewBufferString(input), &output, &errors}

	cookie, err := dkr.Prepare(ctx, task)
	if err != nil {
		t.Fatal("Couldn't prepare task test")
	}
	defer cookie.Close(ctx)

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

	expect := "italian parsley"
	got := output.String()
	if !strings.Contains(got, expect) {
		t.Errorf("Test expected output to contain '%s', got '%s'", expect, got)
	}
}

func TestRegistry(t *testing.T) {
	image := "fnproject/fn-test-utils"

	sizer, err := CheckRegistry(context.Background(), image, docker.AuthConfiguration{})
	if err != nil {
		t.Fatal("expected registry check not to fail, got:", err)
	}

	size, err := sizer.Size()
	if err != nil {
		t.Fatal("expected sizer not to fail, got:", err)
	}

	if size <= 0 {
		t.Fatal("expected positive size for image that exists, got size:", size)
	}
}
