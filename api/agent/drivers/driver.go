// Interface for all container drivers

package drivers

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/fnproject/fn/api/common"
)

// A DriverCookie identifies a unique request to run a task.
//
// Clients should always call Close() on a DriverCookie after they are done
// with it.
type Cookie interface {
	// Close should clean up any resources the cookie was using, or was going to use.
	Close(ctx context.Context) error

	// Run should execute task on the implementation.
	// RunResult captures the result of task execution. This means if task
	// execution fails due to a problem in the task, Run() MUST return a valid
	// RunResult and nil as the error. The RunResult's Error() and Status()
	// should be used to indicate failure.
	// If the implementation itself suffers problems (lost of network, out of
	// disk etc.), a nil RunResult and an error message is preferred.
	//
	// Run() MUST monitor the context. task cancellation is indicated by
	// cancelling the context.
	Run(ctx context.Context) (WaitResult, error)

	// Freeze the container to pause running processes
	Freeze(ctx context.Context) error

	// Unfreeze a frozen container to unpause frozen processes
	Unfreeze(ctx context.Context) error

	// Fetch driver specific container configuration. Use this to
	// access the container create options. If Driver.Prepare() is not
	// yet called with the cookie, then this can be used to modify container
	// create options.
	ContainerOptions() interface{}
}

type WaitResult interface {
	// Wait may be called to await the result of a container's execution. If the
	// provided context is canceled and the container does not return first, the
	// resulting status will be 'canceled'. If the provided context times out
	// then the resulting status will be 'timeout'.
	Wait(context.Context) RunResult
}

type Driver interface {
	// Create a new cookie with defaults and/or settings from container task.
	// Callers should Close the cookie regardless of whether they prepare or run it.
	CreateCookie(ctx context.Context, task ContainerTask) (Cookie, error)

	// PrepareCookie can be used in order to do any preparation that a specific driver
	// may need to do before running the task, and can be useful to put
	// preparation that the task can recover from into (i.e. if pulling an image
	// fails because a registry is down, the task doesn't need to be failed).  It
	// returns a cookie that can be used to execute the task.
	// Callers should Close the cookie regardless of whether they run it.
	//
	// The returned cookie should respect the task's timeout when it is run.
	PrepareCookie(ctx context.Context, cookie Cookie) error

	// close & shutdown the driver
	Close() error
}

// RunResult indicates only the final state of the task.
type RunResult interface {
	// Error is an actionable/checkable error from the container, nil if
	// Status() returns "success", otherwise non-nil
	Error() error

	// Status should return the current status of the task.
	// Only valid options are {"error", "success", "timeout", "killed", "cancelled"}.
	Status() string
}

// Logger Tags for container
type LoggerTag struct {
	Name  string
	Value string
}

// Logger Configuration for container
type LoggerConfig struct {
	// Log Sink URL
	URL string

	// Log Tag Pairs
	Tags []LoggerTag
}

// The ContainerTask interface guides container execution across a wide variety of
// container oriented runtimes.
type ContainerTask interface {
	// Command returns the command to run within the container.
	Command() string

	// EnvVars returns environment variable key-value pairs.
	EnvVars() map[string]string

	// Input feeds the container with data
	Input() io.Reader

	// The id to assign the container
	Id() string

	// Image returns the runtime specific image to run.
	Image() string

	// Timeout specifies the maximum time a task is allowed to run. Return 0 to let it run forever.
	Timeout() time.Duration

	// Driver will write output log from task execution to these writers. Must be
	// non-nil. Use io.Discard if log is irrelevant.
	Logger() (stdout, stderr io.Writer)

	// WriteStat writes a single Stat, implementation need not be thread safe.
	WriteStat(context.Context, Stat)

	// Volumes returns an array of 2-element tuples indicating storage volume mounts.
	// The first element is the path on the host, and the second element is the
	// path in the container.
	Volumes() [][2]string

	// Memory determines the max amount of RAM given to the container to use.
	// 0 is unlimited.
	Memory() uint64

	// CPUs in milli CPU units
	CPUs() uint64

	// Filesystem size limit for the container, in megabytes.
	FsSize() uint64

	// Tmpfs Filesystem size limit for the container, in megabytes.
	TmpFsSize() uint64

	// WorkDir returns the working directory to use for the task. Empty string
	// leaves it unset.
	WorkDir() string

	// Logger Config to use in driver
	LoggerConfig() LoggerConfig

	// Close is used to perform cleanup after task execution.
	// Close should be safe to call multiple times.
	Close()

	// Extensions are extra driver specific configuration options. They should be
	// more specific but it's easier to be lazy.
	Extensions() map[string]string

	// UDSAgentPath to use to configure the unix domain socket.
	// This is the mount point relative to the agent
	// abstractions have leaked so bad at this point it's a monsoon.
	UDSAgentPath() string

	// UDSDockerPath to use to configure the unix domain socket. the drivers
	// This is the mount point relative to the docker host.
	UDSDockerPath() string

	// UDSDockerDest is the destination mount point for uds path. it is the path
	// of the directory where the sock file resides inside of the container.
	UDSDockerDest() string
}

// Stat is a bucket of stats from a driver at a point in time for a certain task.
type Stat struct {
	Timestamp common.DateTime   `json:"timestamp"`
	Metrics   map[string]uint64 `json:"metrics"`
}

// Stats is a list of Stat, notably implements sql.Valuer
type Stats []Stat

// implements sql.Valuer, returning a string
func (s Stats) Value() (driver.Value, error) {
	if len(s) < 1 {
		return driver.Value(string("")), nil
	}
	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(s)
	// return a string type
	return driver.Value(b.String()), err
}

// implements sql.Scanner
func (s *Stats) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}
	bv, err := driver.String.ConvertValue(value)
	if err == nil {
		var b []byte
		switch x := bv.(type) {
		case []byte:
			b = x
		case string:
			b = []byte(x)
		}

		if len(b) > 0 {
			return json.Unmarshal(b, s)
		}

		*s = nil
		return nil
	}

	// otherwise, return an error
	return fmt.Errorf("stats invalid db format: %T %T value, err: %v", value, bv, err)
}

// TODO: ensure some type is applied to these statuses.
const (
	// task statuses
	StatusRunning   = "running"
	StatusSuccess   = "success"
	StatusError     = "error"
	StatusTimeout   = "timeout"
	StatusKilled    = "killed"
	StatusCancelled = "cancelled"

	defaultDomain = "docker.io"
)

type Config struct {
	// TODO this should all be driver-specific config and not in the
	// driver package itself. fix if we ever one day try something else
	Docker               string `json:"docker"`
	DockerNetworks       string `json:"docker_networks"`
	DockerLoadFile       string `json:"docker_load_file"`
	ServerVersion        string `json:"server_version"`
	PreForkPoolSize      uint64 `json:"pre_fork_pool_size"`
	PreForkImage         string `json:"pre_fork_image"`
	PreForkCmd           string `json:"pre_fork_cmd"`
	PreForkUseOnce       uint64 `json:"pre_fork_use_once"`
	PreForkNetworks      string `json:"pre_fork_networks"`
	MaxTmpFsInodes       uint64 `json:"max_tmpfs_inodes"`
	EnableReadOnlyRootFs bool   `json:"enable_readonly_rootfs"`
	EnableTini           bool   `json:"enable_tini"`
	MaxRetries           uint64 `json:"max_retries"`
}

func average(samples []Stat) (Stat, bool) {
	l := len(samples)
	if l == 0 {
		return Stat{}, false
	} else if l == 1 {
		return samples[0], true
	}

	s := Stat{
		Metrics: samples[0].Metrics, // Recycle Metrics map from first sample
	}
	t := time.Time(samples[0].Timestamp).UnixNano() / int64(l)
	for _, sample := range samples[1:] {
		t += time.Time(sample.Timestamp).UnixNano() / int64(l)
		for k, v := range sample.Metrics {
			s.Metrics[k] += v
		}
	}

	s.Timestamp = common.DateTime(time.Unix(0, t))
	for k, v := range s.Metrics {
		s.Metrics[k] = v / uint64(l)
	}
	return s, true
}

// Decimate will down sample to a max number of points in a given sample by
// averaging samples together. i.e. max=240, if we have 240 samples, return
// them all, if we have 480 samples, every 2 samples average them (and time
// distance), and return 240 samples. This is relatively naive and if len(in) >
// max, <= max points will be returned, not necessarily max: length(out) =
// ceil(length(in)/max) -- feel free to fix this, setting a relatively high max
// will allow good enough granularity at higher lengths, i.e. for max of 1 hour
// tasks, sampling every 1s, decimate will return 15s samples if max=240.
// Large gaps in time between samples (a factor > (last-start)/max) will result
// in a shorter list being returned to account for lost samples.
// Decimate will modify the input list for efficiency, it is not copy safe.
// Input must be sorted by timestamp or this will fail gloriously.
func Decimate(maxSamples int, stats []Stat) []Stat {
	if len(stats) <= maxSamples {
		return stats
	} else if maxSamples <= 0 { // protect from nefarious input
		return nil
	}

	start := time.Time(stats[0].Timestamp)
	window := time.Time(stats[len(stats)-1].Timestamp).Sub(start) / time.Duration(maxSamples)

	nextEntry, current := 0, start // nextEntry is the index tracking next Stats record location
	for x := 0; x < len(stats); {
		isLastEntry := nextEntry == maxSamples-1 // Last bin is larger than others to handle imprecision

		var samples []Stat
		for offset := 0; x+offset < len(stats); offset++ { // Iterate through samples until out of window
			if !isLastEntry && time.Time(stats[x+offset].Timestamp).After(current.Add(window)) {
				break
			}
			samples = stats[x : x+offset+1]
		}

		x += len(samples)                      // Skip # of samples for next window
		if entry, ok := average(samples); ok { // Only record Stat if 1+ samples exist
			stats[nextEntry] = entry
			nextEntry++
		}

		current = current.Add(window)
	}
	return stats[:nextEntry] // Return slice of []Stats that was modified with averages
}

// https://github.com/fsouza/go-dockerclient/blob/master/misc.go#L166
func parseRepositoryTag(repoTag string) (repository string, tag string) {
	parts := strings.SplitN(repoTag, "@", 2)
	repoTag = parts[0]
	n := strings.LastIndex(repoTag, ":")
	if n < 0 {
		return repoTag, ""
	}
	if tag := repoTag[n+1:]; !strings.Contains(tag, "/") {
		return repoTag[:n], tag
	}
	return repoTag, ""
}

func ParseImage(image string) (registry, repo, tag string) {
	// registry = defaultDomain // TODO uneasy about this, but it seems wise
	repo, tag = parseRepositoryTag(image)
	// Officially sanctioned at https://github.com/moby/moby/blob/4f0d95fa6ee7f865597c03b9e63702cdcb0f7067/registry/service.go#L155 to deal with "Official Repositories".
	// Without this, token auth fails.
	// Registries must exist at root (https://github.com/moby/moby/issues/7067#issuecomment-54302847)
	// This cannot support the `library/` shortcut for private registries.
	parts := strings.SplitN(repo, "/", 2)
	switch len(parts) {
	case 1:
		repo = "library/" + repo
	default:
		// detect if repo has a hostname, otherwise leave it
		if strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") || parts[0] == "localhost" {
			registry = parts[0]
			repo = parts[1]
		}
	}

	if tag == "" {
		tag = "latest"
	}

	return registry, repo, tag
}
