package tests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"testing"
	"time"

	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/runnerpool"
)

func callFN(ctx context.Context, u string, content io.Reader, output io.Writer) (*http.Response, error) {
	method := "POST"

	req, err := http.NewRequest(method, u, content)
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

	resp, err := callFN(ctx, u.String(), content, output)
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
func (c *myCall) SlotHashId() string                  { return "" }
func (c *myCall) Extensions() map[string]string       { return nil }
func (c *myCall) RequestBody() io.ReadCloser          { return nil }
func (c *myCall) ResponseWriter() http.ResponseWriter { return nil }
func (c *myCall) StdErr() io.ReadWriteCloser          { return nil }
func (c *myCall) Model() *models.Call                 { return nil }

func TestExecuteRunnerStatus(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var zoo myCall

	pool, err := NewSystemTestNodePool()
	if err != nil {
		t.Fatalf("Creating Node Pool failed %v", err)
	}

	runners, err := pool.Runners(&zoo)
	if err != nil {
		t.Fatalf("Getting Runners from Pool failed %v", err)
	}
	if len(runners) == 0 {
		t.Fatalf("Getting Runners from Pool failed no-runners")
	}

	concurrency := 10
	res := make(chan *runnerpool.RunnerStatus, concurrency*len(runners))

	for _, runner := range runners {
		for i := 0; i < concurrency; i++ {
			go func(dest runnerpool.Runner) {
				status, err := dest.Status(ctx)
				if err != nil {
					t.Fatalf("Runners Status failed for %v err=%v", dest.Address(), err)
				}
				if status == nil || status.StatusFailed {
					t.Fatalf("Runners Status not OK for %v %v", dest.Address(), status)
				}
				t.Logf("Runner %v got Status=%+v", dest.Address(), status)
				res <- status
			}(runner)
		}
	}

	lookup := make(map[string][]*runnerpool.RunnerStatus)

	for i := 0; i < concurrency*len(runners); i++ {
		status := <-res
		lookup[status.StatusId] = append(lookup[status.StatusId], status)
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
		t.Logf("Runner %v got Status=%+v", dest.Address(), status)
		_, ok := lookup[status.StatusId]
		if ok {
			t.Fatalf("Runners Status did not return fresh status id %v %v", dest.Address(), status)
		}
	}

}
