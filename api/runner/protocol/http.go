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

	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/runner/task"
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

func (p *HTTPProtocol) Dispatch(ctx context.Context, cfg *task.Config) error {
	var retErr error
	done := make(chan struct{})
	go func() {
		// TODO not okay. plumb content-length from req into cfg..
		var body bytes.Buffer
		io.Copy(&body, cfg.Stdin)
		req, err := http.NewRequest("GET", "/", &body)
		if err != nil {
			retErr = err
			return
		}
		for k, v := range cfg.Env {
			req.Header.Set(k, v)
		}
		req.Header.Set("Content-Length", fmt.Sprint(body.Len()))
		req.Header.Set("Task-ID", cfg.ID)
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

		io.Copy(cfg.Stdout, res.Body)
		done <- struct{}{}
	}()

	timeout := time.After(cfg.Timeout)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timeout:
		return models.ErrRunnerTimeout
	case <-done:
		return retErr
	}
}
