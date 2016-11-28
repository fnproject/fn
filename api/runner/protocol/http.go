package protocol

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/runner/task"
)

// HTTPProtocol converts stdin/stdout streams into HTTP/1.1 compliant
// communication. It relies on Content-Length to know when to stop reading from
// containers stdout. It also mandates valid HTTP headers back and forth, thus
// returning errors in case of parsing problems.
type HTTPProtocol struct {
	in  io.Writer
	out io.Reader
}

func (p *HTTPProtocol) IsStreamable() bool {
	return true
}

func (p *HTTPProtocol) Dispatch(ctx context.Context, t task.Request) error {
	var retErr error
	done := make(chan struct{})
	go func() {
		var body bytes.Buffer
		io.Copy(&body, t.Config.Stdin)
		req, err := http.NewRequest("GET", "/", &body)
		if err != nil {
			retErr = err
			return
		}
		for k, v := range t.Config.Env {
			req.Header.Set(k, v)
		}
		req.Header.Set("Content-Length", fmt.Sprint(body.Len()))
		req.Header.Set("Task-ID", t.Config.ID)
		raw, err := httputil.DumpRequest(req, true)
		if err != nil {
			retErr = err
			return
		}
		p.in.Write(raw)

		res, err := http.ReadResponse(bufio.NewReader(p.out), req)
		if err != nil {
			retErr = err
			return
		}

		io.Copy(t.Config.Stdout, res.Body)
		done <- struct{}{}
	}()

	timeout := time.After(t.Config.Timeout)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timeout:
		return models.ErrRunnerTimeout
	case <-done:
		return retErr
	}
}
