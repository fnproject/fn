package task

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/runner/drivers"
	"github.com/go-openapi/strfmt"
)

// TODO this whole package should be hanged, drawn & quartered

type Config struct {
	ID           string
	AppName      string
	Path         string
	Image        string
	Timeout      time.Duration
	IdleTimeout  time.Duration
	Memory       uint64
	BaseEnv      map[string]string // only app & route config vals [for hot]
	Env          map[string]string // includes BaseEnv
	Format       string
	ReceivedTime time.Time
	// Ready is used to await the first pull
	Ready chan struct{}

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.WriteCloser // closer for flushy poo
}

// TODO Task & Config should be merged
func TaskFromConfig(cfg *Config) *models.Task {
	return &models.Task{
		IDStatus:    models.IDStatus{ID: cfg.ID},
		AppName:     cfg.AppName,
		Path:        cfg.Path,
		Image:       cfg.Image,
		Timeout:     int32(cfg.Timeout.Seconds()),
		IdleTimeout: int32(cfg.IdleTimeout.Seconds()),
		Memory:      cfg.Memory,
		BaseEnv:     cfg.BaseEnv,
		EnvVars:     cfg.Env,
		// Format:      cfg.Format, TODO plumb this
		CreatedAt: strfmt.DateTime(time.Now()),

		Delay: 0, // TODO not wired to users
		// Payload: stdin
		Priority: new(int32), // 0, TODO not wired atm to users.
	}
}

func ConfigFromTask(t *models.Task) *Config {
	return &Config{
		ID:          t.ID,
		AppName:     t.AppName,
		Path:        t.Path,
		Image:       t.Image,
		Timeout:     time.Duration(t.Timeout) * time.Second,
		IdleTimeout: time.Duration(t.IdleTimeout) * time.Second,
		Memory:      t.Memory,
		BaseEnv:     t.BaseEnv,
		Env:         t.EnvVars,
		Stdin:       strings.NewReader(t.Payload),
		Ready:       make(chan struct{}),
	}
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
