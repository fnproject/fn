package tests

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"path"
	"testing"
	"time"

	"github.com/fnproject/fn/api/models"
)

// We should not be able to invoke a StatusImage
func TestCannotExecuteStatusImage(t *testing.T) {
	if StatusImage == "" {
		t.Skip("no status image defined")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rt := &models.Route{
		Path:   routeName + "yogurt",
		Image:  StatusImage,
		Format: format,
		Memory: memory,
		Type:   typ,
	}

	rt = ensureRoute(t, rt)

	lb, err := LB()
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}
	u := url.URL{
		Scheme: "http",
		Host:   lb,
	}
	u.Path = path.Join(u.Path, "r", appName, rt.Path)

	content := bytes.NewBuffer([]byte(`status`))
	output := &bytes.Buffer{}

	resp, err := callFN(ctx, u.String(), content, output, "POST")
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

	for _, runner := range runners {
		isOK, err := runner.Status(ctx)
		if err != nil {
			t.Fatalf("Runners Status failed for %v err=%v", runner.Address(), err)
		}
		if !isOK {
			t.Fatalf("Runners Status not OK for %v", runner.Address())
		}
	}
}
