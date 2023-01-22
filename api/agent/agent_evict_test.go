package agent

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fnproject/fn/api/agent/drivers"
	_ "github.com/fnproject/fn/api/agent/drivers/docker"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
)

// create a simple non-blocking agent. Non-blocking does not queue, so it's
// easier to test and see if evictions took place.
func getAgentWithDriver() (Agent, drivers.Driver, error) {
	cfg, err := NewConfig()
	if err != nil {
		return nil, nil, err
	}

	// 160MB memory
	cfg.EnableNBResourceTracker = true
	cfg.HotPoll = 20
	cfg.MaxTotalMemory = 160 * 1024 * 1024
	cfg.HotPullTimeout = time.Duration(10000) * time.Millisecond
	cfg.HotStartTimeout = time.Duration(10000) * time.Millisecond

	drv, err := NewDockerDriver(cfg)
	if err != nil {
		return nil, nil, err
	}

	a := New(WithConfig(cfg), WithDockerDriver(drv))
	return a, drv, nil
}

func getAgent() (Agent, error) {
	a, _, err := getAgentWithDriver()
	if err != nil {
		return nil, err
	}
	return a, nil
}

func getHungDocker() (*httptest.Server, func()) {
	hung, cancel := context.WithCancel(context.Background())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// version check seem to have a sane timeout in docker, let's serve this, then stop
		if r.URL.String() == "/v2/" {
			w.WriteHeader(200)
			return
		}
		<-hung.Done()
	}))

	closer := func() {
		cancel()
		srv.Close()
	}

	return srv, closer
}

func getApp() *models.App {
	return &models.App{ID: id.New().String()}
}

func getFn(initDelayMsecs int) *models.Fn {
	fn := &models.Fn{
		ID:    id.New().String(),
		Image: "fnproject/fn-test-utils",
		ResourceConfig: models.ResourceConfig{
			Timeout:     10,
			IdleTimeout: 60,
			Memory:      128, // only 1 fit in 160MB
		},
	}
	if initDelayMsecs > 0 {
		fn.Config = models.Config{"ENABLE_INIT_DELAY_MSEC": strconv.FormatUint(uint64(initDelayMsecs), 10)}
	}
	return fn
}

// simple GetCall/Submit combo.
func execFn(input string, fn *models.Fn, app *models.App, a Agent, tmsec int) error {

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(tmsec)*time.Millisecond)
	defer cancel()

	url := "http://127.0.0.1:8080/invoke/" + fn.ID

	req, err := http.NewRequest("GET", url, &dummyReader{Reader: strings.NewReader(input)})
	if err != nil {
		return err
	}

	req = req.WithContext(ctx)

	var out bytes.Buffer
	callI, err := a.GetCall(FromHTTPFnRequest(app, fn, req), WithWriter(&out))
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}

	fmt.Println("~~before submit")
	err = a.Submit(callI)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func TestBadContainer1(t *testing.T) {
	a, err := getAgent()
	if err != nil {
		t.Fatal("cannot create agent")
	}
	defer checkClose(t, a)

	fn := getFn(0)
	fn.Config = models.Config{"ENABLE_INIT_EXIT": "0"}

	err = execFn(`{"sleepTime": 8000}`, fn, getApp(), a, 20000)
	if err != models.ErrContainerInitFail {
		t.Fatalf("submit unexpected error! %v", err)
	}
}

func TestBadContainer2(t *testing.T) {
	a, err := getAgent()
	if err != nil {
		t.Fatal("cannot create agent")
	}
	defer checkClose(t, a)

	fn := getFn(0)
	fn.Config = models.Config{"ENABLE_INIT_EXIT": "0", "ENABLE_INIT_DELAY_MSEC": "200"}

	err = execFn(`{"sleepTime": 8000}`, fn, getApp(), a, 20000)
	if err != models.ErrContainerInitFail {
		t.Fatalf("submit unexpected error! %v", err)
	}
}

func TestBadContainer3(t *testing.T) {
	a, err := getAgent()
	if err != nil {
		t.Fatal("cannot create agent")
	}
	defer checkClose(t, a)

	fn := getFn(0)

	err = execFn(`{"isCrash": true }`, fn, getApp(), a, 20000)
	if err != models.ErrFunctionResponse {
		t.Fatalf("submit unexpected error! %v", err)
	}
}

func TestBadContainer4(t *testing.T) {
	a, err := getAgent()
	if err != nil {
		t.Fatal("cannot create agent")
	}
	defer checkClose(t, a)

	fn := getFn(0)

	err = execFn(`{"isExit": true, "exitCode": 0 }`, fn, getApp(), a, 20000)
	if err != models.ErrFunctionResponse {
		t.Fatalf("submit unexpected error! %v", err)
	}
}

// Eviction will NOT take place since the first container is busy
func TestPlainNoEvict(t *testing.T) {
	a, err := getAgent()
	if err != nil {
		t.Fatal("cannot create agent")
	}
	defer checkClose(t, a)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		err := execFn(`{"sleepTime": 8000}`, getFn(0), getApp(), a, 20000)
		if err != nil {
			t.Fatalf("submit should not error! %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		time.Sleep(3000 * time.Millisecond)
		err := execFn(`{"sleepTime": 0}`, getFn(0), getApp(), a, 20000)
		if err != models.ErrCallTimeoutServerBusy {
			t.Fatalf("unexpected error %v", err)
		}
	}()

	wg.Wait()
}

// Eviction will take place since the first container is idle
func TestPlainDoEvict(t *testing.T) {
	a, err := getAgent()
	if err != nil {
		t.Fatal("cannot create agent")
	}
	defer checkClose(t, a)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		err := execFn(`{"sleepTime": 0}`, getFn(0), getApp(), a, 20000)
		if err != nil {
			t.Fatalf("submit should not error! %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		time.Sleep(3000 * time.Millisecond)
		err := execFn(`{"sleepTime": 0}`, getFn(0), getApp(), a, 20000)
		if err != nil {
			t.Fatalf("submit should not error! %v", err)
		}
	}()

	wg.Wait()
}

func TestHungFDKNoEvict(t *testing.T) {
	a, err := getAgent()
	if err != nil {
		t.Fatal("cannot create agent")
	}
	defer checkClose(t, a)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		err := execFn(`{"sleepTime": 0}`, getFn(11000), getApp(), a, 20000)
		if err != models.ErrContainerInitTimeout {
			t.Fatalf("submit unexpected error! %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		time.Sleep(3000 * time.Millisecond)
		err := execFn(`{"sleepTime": 0}`, getFn(0), getApp(), a, 20000)
		if err != models.ErrCallTimeoutServerBusy {
			t.Fatalf("unexpected error %v", err)
		}
	}()

	wg.Wait()
}

func TestDockerPullHungNoEvict(t *testing.T) {
	dockerSrv, dockerCancel := getHungDocker()
	defer dockerCancel()

	a, err := getAgent()
	if err != nil {
		t.Fatal("cannot create agent")
	}
	defer checkClose(t, a)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()

		fn := getFn(0)
		fn.Image = strings.TrimPrefix(dockerSrv.URL, "http://") + "/" + fn.Image

		err := execFn(`{"sleepTime": 0}`, fn, getApp(), a, 20000)
		if err != models.ErrDockerPullTimeout {
			t.Fatalf("unexpected error %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		time.Sleep(3000 * time.Millisecond)
		err := execFn(`{"sleepTime": 0}`, getFn(0), getApp(), a, 20000)
		if err != models.ErrCallTimeoutServerBusy {
			t.Fatalf("unexpected error %v", err)
		}
	}()

	wg.Wait()

}
