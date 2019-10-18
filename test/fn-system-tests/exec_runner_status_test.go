package tests

import (
	"bytes"
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"testing"
	"time"

	runner "github.com/fnproject/fn/api/agent/grpc"
	"google.golang.org/grpc"

	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/runnerpool"
	pb_empty "github.com/golang/protobuf/ptypes/empty"
	pb_struct "github.com/golang/protobuf/ptypes/struct"
)

func callFN(ctx context.Context, u string, content io.Reader, output io.Writer, invokeType string) (*http.Response, error) {
	method := "POST"

	req, err := http.NewRequest(method, u, content)
	req.Header.Set("Fn-Invoke-Type", invokeType)
	if err != nil {
		return nil, fmt.Errorf("error running fn: %s", err)
	}
	req.Header.Set("Content-Type", "application/json")

	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error running fn: %s", err)
	}

	io.Copy(output, resp.Body)

	return resp, nil
}

// We should not be able to invoke a StatusImage
func TestCannotExecuteStatusImage(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()

	if StatusImage == "" {
		t.Skip("no status image defined")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	app := &models.App{Name: id.New().String()}
	app = ensureApp(t, app)

	fn := &models.Fn{
		AppID: app.ID,
		Name:  id.New().String(),
		Image: StatusImage,
		ResourceConfig: models.ResourceConfig{
			Memory: memory,
		},
	}
	fn = ensureFn(t, fn)

	lb, err := LB()
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}
	u := url.URL{
		Scheme: "http",
		Host:   lb,
	}
	u.Path = path.Join(u.Path, "invoke", fn.ID)

	content := bytes.NewBuffer([]byte(`status`))
	output := &bytes.Buffer{}

	resp, err := callFN(ctx, u.String(), content, output, models.TypeSync)
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("StatusCode check failed on %v", resp.StatusCode)
	}
}

// Some dummy RunnerCall implementation
type myCall struct{}

// implements RunnerCall
func (c *myCall) SlotHashId() string                   { return "" }
func (c *myCall) Extensions() map[string]string        { return nil }
func (c *myCall) RequestBody() io.ReadCloser           { return nil }
func (c *myCall) ResponseWriter() http.ResponseWriter  { return nil }
func (c *myCall) StdErr() io.ReadWriteCloser           { return nil }
func (c *myCall) Model() *models.Call                  { return nil }
func (c *myCall) GetUserExecutionTime() *time.Duration { return nil }
func (c *myCall) AddUserExecutionTime(time.Duration)   {}

func TestExecuteRunnerStatusConcurrent(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var zoo myCall

	pool, err := NewSystemTestNodePool()
	if err != nil {
		t.Fatalf("Creating Node Pool failed %v", err)
	}

	runners, err := pool.Runners(context.Background(), &zoo)
	if err != nil {
		t.Fatalf("Getting Runners from Pool failed %v", err)
	}
	if len(runners) == 0 {
		t.Fatalf("Getting Runners from Pool failed no-runners")
	}

	concurrency := 10
	res := make(chan *runnerpool.RunnerStatus, concurrency*len(runners))
	errs := make(chan error, concurrency*len(runners))

	for _, runner := range runners {
		for i := 0; i < concurrency; i++ {
			go func(dest runnerpool.Runner) {
				status, err := dest.Status(ctx)
				if err != nil {
					errs <- err
				} else {
					t.Logf("Runner %v got Status=%+v", dest.Address(), status)
					res <- status
				}
			}(runner)
		}
	}

	lookup := make(map[string][]*runnerpool.RunnerStatus)

	for i := 0; i < concurrency*len(runners); i++ {
		select {
		case status := <-res:
			if status == nil || status.StatusFailed {
				t.Fatalf("Runners Status not OK for %+v", status)
			}
			lookup[status.StatusId] = append(lookup[status.StatusId], status)
		case err := <-errs:
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	// WARNING: Possibly flappy test below. Might need to relax the numbers below.
	// Why 3? We have a idleTimeout + gracePeriod = 1.5 secs (for cache timeout) for status calls.
	// This normally should easily serve all the queries above. (We have 3 runners, each should
	// easily take on 10 status calls for that period.
	if len(lookup) > 3 {
		for key, arr := range lookup {
			t.Fatalf("key=%v count=%v", key, len(arr))
		}
	}

	// delay
	time.Sleep(time.Duration(2 * time.Second))

	// now we should get fresh data
	for _, dest := range runners {
		status, err := dest.Status(ctx)
		if err != nil {
			t.Fatalf("Runners Status failed for %v err=%v", dest.Address(), err)
		}
		if status == nil || status.StatusFailed {
			t.Fatalf("Runners Status not OK for %v %v", dest.Address(), status)
		}
		if status.IsNetworkDisabled {
			t.Fatalf("Runners Status should have network enabled %v %v", dest.Address(), status)
		}
		t.Logf("Runner %v got Status=%+v", dest.Address(), status)
		_, ok := lookup[status.StatusId]
		if ok {
			t.Fatalf("Runners Status did not return fresh status id %v %v", dest.Address(), status)
		}
	}

}

// Test faulty runner pool, which is waiting on a non-existent docker network
func TestExecuteRunnerStatusNoNet(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var zoo myCall

	pool, err := NewSystemTestNodePoolNoNet()
	if err != nil {
		t.Fatalf("Creating Node Pool failed %v", err)
	}

	runners, err := pool.Runners(context.Background(), &zoo)
	if err != nil {
		t.Fatalf("Getting Runners from Pool failed %v", err)
	}
	if len(runners) == 0 {
		t.Fatalf("Getting Runners from Pool failed no-runners")
	}

	for _, dest := range runners {
		status, err := dest.Status(ctx)
		if err != nil {
			t.Fatalf("Runners Status failed for %v err=%v", dest.Address(), err)
		}
		if status == nil || status.StatusFailed {
			t.Fatalf("Runners Status not OK for %v %v", dest.Address(), status)
		}
		if !status.IsNetworkDisabled {
			t.Fatalf("Runners Status should have NO network enabled %v %v", dest.Address(), status)
		}
		t.Logf("Runner %v got Status=%+v", dest.Address(), status)
	}

	f, err := os.Create(StatusBarrierFile)
	if err != nil {
		t.Fatalf("create file=%v failed err=%v", StatusBarrierFile, err)
	}
	f.Close()

	// Let status hc caches expire.
	select {
	case <-time.After(time.Duration(2 * time.Second)):
	case <-ctx.Done():
		t.Fatal("Timeout")
	}

	for _, dest := range runners {
		status, err := dest.Status(ctx)
		if err != nil {
			t.Fatalf("Runners Status failed for %v err=%v", dest.Address(), err)
		}
		if status == nil || status.StatusFailed {
			t.Fatalf("Runners Status not OK for %v %v", dest.Address(), status)
		}
		if status.IsNetworkDisabled {
			t.Fatalf("Runners Status should have network enabled %v %v", dest.Address(), status)
		}
		t.Logf("Runner %v got Status=%+v", dest.Address(), status)
	}

}

// Test custom health checker scenarios
func TestCustomHealthChecker(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	r := "127.0.0.1:9191"
	conn, err := grpc.Dial(r, grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial into runner %s due to err=%+v", r, err)
	}
	client := runner.NewRunnerProtocolClient(conn)
	status, err := client.Status(ctx, &pb_empty.Empty{})
	if err != nil {
		t.Fatalf("Status check failed due to err=%+v", err)
	}
	if status.CustomStatus == nil || status.CustomStatus["custom"] != "works" {
		t.Fatalf("Custom status did not match expected status actual=%+v", status.CustomStatus)
	}

	// Let status hc caches expire.
	select {
	case <-time.After(time.Duration(2 * time.Second)):
	case <-ctx.Done():
		t.Fatal("Timeout")
	}
	shouldCustomHealthCheckerFail = true
	defer func() {
		// Reset test state
		// Ensure status cache expires
		shouldCustomHealthCheckerFail = false
		time.Sleep(2 * time.Second)
	}()
	status, err = client.Status(ctx, &pb_empty.Empty{})
	if err != nil {
		t.Fatalf("Status check failed due to err=%+v", err)
	}
	if status.ErrorCode != 450 {
		t.Fatalf("Custom status check should have failed with 450 but actual status was %+v", status)
	}
}

func TestConfigureRunner(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	r := "127.0.0.1:9193"
	conn, err := grpc.Dial(r, grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial into runner %s due to err=%+v", r, err)
	}
	client := runner.NewRunnerProtocolClient(conn)

	var config = make(map[string]string)
	config["domain"] = "unix"
	config["company"] = "fn"
	_, err = client.ConfigureRunner(ctx, &runner.ConfigMsg{Config: config})
	if err != nil {
		t.Fatalf("Failed to configure runner due to %+v", err)
	}

	if configureRunnerSetsThis == nil {
		t.Fatal("Configuration was not handled as expected")
	}
	if _, ok := configureRunnerSetsThis["domain"]; !ok {
		t.Fatalf("Configuration was not handled as expected")
	}
}

func TestExampleLogStreamer(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	r := "127.0.0.1:9193"
	conn, err := grpc.Dial(r, grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial into runner %s due to err=%+v", r, err)
	}
	client := runner.NewRunnerProtocolClient(conn)

	logStream, err := client.StreamLogs(ctx)
	if err != nil {
		t.Fatalf("Failed to configure runner due to %+v", err)
	}

	start := runner.LogRequestMsg_Start{}
	ack := runner.LogRequestMsg_Ack{}

	err = logStream.Send(&runner.LogRequestMsg{Body: &runner.LogRequestMsg_Start_{Start: &start}})
	if err != nil {
		t.Fatalf("Failed to send start session %+v", err)
	}

	err = logStream.Send(&runner.LogRequestMsg{Body: &runner.LogRequestMsg_Ack_{Ack: &ack}})
	if err != nil {
		t.Fatalf("Failed to send ack %+v", err)
	}

	resp, err := logStream.Recv()
	if err != nil {
		t.Fatalf("Failed to get logs %+v", err)
	}

	t.Logf("Got log msg %+v", resp)

	cont := resp.Data[0]
	if cont == nil || cont.ApplicationId != "app1" || cont.FunctionId != "fun1" || cont.ContainerId != "container1" {
		t.Fatalf("Bad container data %+v", cont)
	}

	req := cont.Data[0]
	if req == nil || req.RequestId != "101" {
		t.Fatalf("Bad request data %+v", req)
	}

	now := time.Now().UnixNano() / int64(time.Millisecond)

	data := req.Data[0]
	if data == nil || data.Timestamp > now || data.Source != runner.LogResponseMsg_Container_Request_Line_STDOUT {
		t.Fatalf("Bad log data %+v", data)
	}

}

// TestStatus_verifyIO verifies RunnerProtocol.Status2(..) behavior
// Note: this test depends on the default status checker image
// which echoes its input
func TestStatus_verifyIO(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to the first node
	r := runnerGrpcServerAddr(0)
	conn, err := grpc.Dial(r, grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial into runner %s due to err=%+v", r, err)
	}
	client := runner.NewRunnerProtocolClient(conn)
	p := &pb_struct.Struct{
		Fields: map[string]*pb_struct.Value{
			"fake-member-1": {Kind: &pb_struct.Value_StringValue{StringValue: "fake-value-1"}},
			"fake-member-2": {Kind: &pb_struct.Value_StringValue{StringValue: "fake-value-2"}},
		},
	}

	status, err := client.Status2(ctx, p)
	if err != nil {
		t.Fatalf("unexpected Status2 failure %v", err)
	}
	a := assert.New(t)
	expected := `{
            "fake-member-1": "fake-value-1", 
            "fake-member-2": "fake-value-2" 
          }`
	a.JSONEqf(expected, status.GetDetails(), "Status2: mismatch in output from status image %s ", StatusImage)
}
