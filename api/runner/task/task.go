package task

import (
	"context"
	"io"
	"time"

	"github.com/iron-io/runner/drivers"
)

type Config struct {
	ID                  string
	Path                string
	Image               string
	Timeout             time.Duration
	IdleTimeout         time.Duration
	AppName             string
	Memory              uint64
	Env                 map[string]string
	Format              string
	MaxConcurrency      int

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Request stores the task to be executed by the common concurrency stream,
// whatever type the ask actually is, either sync or async. It holds in itself
// the channel to return its response to its caller.
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
