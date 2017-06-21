package task

import (
	"context"
	"io"
	"time"

	"gitlab-odx.oracle.com/odx/functions/api/runner/drivers"
)

type Config struct {
	ID           string
	Path         string
	Image        string
	Timeout      time.Duration
	IdleTimeout  time.Duration
	AppName      string
	Memory       uint64
	Env          map[string]string
	Format       string
	ReceivedTime time.Time
	// Ready is used to await the first pull
	Ready chan struct{}

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.WriteCloser // closer for flushy poo
}

// Request stores the task to be executed, It holds in itself the channel to
// return its response to its caller.
type Request struct {
	Ctx      context.Context
	Config   *Config
	Response chan Response
}

// Response holds the response metainformation of a Request
type Response struct {
	Result drivers.RunResult
	Err    error
}
