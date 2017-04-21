// Copyright 2016 Iron.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Interface for all container drivers

package drivers

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"code.cloudfoundry.org/bytefmt"
)

// A DriverCookie identifies a unique request to run a task.
//
// Clients should always call Close() on a DriverCookie after they are done
// with it.
type Cookie interface {
	io.Closer

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
	Run(ctx context.Context) (RunResult, error)
}

type Driver interface {
	// Prepare can be used in order to do any preparation that a specific driver
	// may need to do before running the task, and can be useful to put
	// preparation that the task can recover from into (i.e. if pulling an image
	// fails because a registry is down, the task doesn't need to be failed).  It
	// returns a cookie that can be used to execute the task.
	// Callers should Close the cookie regardless of whether they run it.
	//
	// The returned cookie should respect the task's timeout when it is run.
	Prepare(ctx context.Context, task ContainerTask) (Cookie, error)
}

// RunResult indicates only the final state of the task.
type RunResult interface {
	// Error is an actionable/checkable error from the container.
	error

	// Status should return the current status of the task.
	// Only valid options are {"error", "success", "timeout", "killed", "cancelled"}.
	Status() string
}

// The ContainerTask interface guides task execution across a wide variety of
// container oriented runtimes.
// This interface is unstable.
//
// FIXME: This interface is large, and it is currently a little Docker specific.
type ContainerTask interface {
	// Command returns the command to run within the container.
	Command() string
	// EnvVars returns environment variable key-value pairs.
	EnvVars() map[string]string
	// Input feeds the container with data
	Input() io.Reader
	// Labels returns container label key-value pairs.
	Labels() map[string]string
	Id() string
	// Image returns the runtime specific image to run.
	Image() string
	// Timeout specifies the maximum time a task is allowed to run. Return 0 to let it run forever.
	Timeout() time.Duration
	// Driver will write output log from task execution to these writers. Must be
	// non-nil. Use io.Discard if log is irrelevant.
	Logger() (stdout, stderr io.Writer)
	// WriteStat writes a single Stat, implementation need not be thread safe.
	WriteStat(Stat)
	// Volumes returns an array of 2-element tuples indicating storage volume mounts.
	// The first element is the path on the host, and the second element is the
	// path in the container.
	Volumes() [][2]string
	// WorkDir returns the working directory to use for the task. Empty string
	// leaves it unset.
	WorkDir() string

	// Close is used to perform cleanup after task execution.
	// Close should be safe to call multiple times.
	Close()
}

// Stat is a bucket of stats from a driver at a point in time for a certain task.
type Stat struct {
	Timestamp time.Time
	Metrics   map[string]uint64
}

// Set of acceptable errors coming from container engines to TaskRunner
var (
	// ErrOutOfMemory for OOM in container engine
	ErrOutOfMemory = userError(errors.New("out of memory error"))
)

// TODO agent.UserError should be elsewhere
func userError(err error) error { return &ue{err} }

type ue struct {
	error
}

func (u *ue) UserVisible() bool { return true }

// TODO: ensure some type is applied to these statuses.
const (
	// task statuses
	StatusRunning   = "running"
	StatusSuccess   = "success"
	StatusError     = "error"
	StatusTimeout   = "timeout"
	StatusKilled    = "killed"
	StatusCancelled = "cancelled"
)

// Allows us to implement custom unmarshaling of JSON and envconfig.
type Memory uint64

func (m *Memory) Unmarshal(s string) error {
	temp, err := bytefmt.ToBytes(s)
	if err != nil {
		return err
	}

	*m = Memory(temp)
	return nil
}

func (m *Memory) UnmarshalJSON(p []byte) error {
	temp, err := bytefmt.ToBytes(string(p))
	if err != nil {
		return err
	}

	*m = Memory(temp)
	return nil
}

type Config struct {
	Docker    string `json:"docker" envconfig:"default=unix:///var/run/docker.sock,DOCKER"`
	Memory    Memory `json:"memory" envconfig:"default=256M,MEMORY_PER_JOB"`
	CPUShares int64  `json:"cpu_shares" envconfig:"default=2,CPU_SHARES"`
}

// for tests
func DefaultConfig() Config {
	return Config{
		Docker:    "unix:///var/run/docker.sock",
		Memory:    256 * 1024 * 1024,
		CPUShares: 0,
	}
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
	t := samples[0].Timestamp.UnixNano() / int64(l)
	for _, sample := range samples[1:] {
		t += sample.Timestamp.UnixNano() / int64(l)
		for k, v := range sample.Metrics {
			s.Metrics[k] += v
		}
	}

	s.Timestamp = time.Unix(0, t)
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

	start := stats[0].Timestamp
	window := stats[len(stats)-1].Timestamp.Sub(start) / time.Duration(maxSamples)

	nextEntry, current := 0, start // nextEntry is the index tracking next Stats record location
	for x := 0; x < len(stats); {
		isLastEntry := nextEntry == maxSamples-1 // Last bin is larger than others to handle imprecision

		var samples []Stat
		for offset := 0; x+offset < len(stats); offset++ { // Iterate through samples until out of window
			if !isLastEntry && stats[x+offset].Timestamp.After(current.Add(window)) {
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
	repo, tag = parseRepositoryTag(image)
	// Officially sanctioned at https://github.com/docker/docker/blob/master/registry/session.go#L319 to deal with "Official Repositories".
	// Without this, token auth fails.
	// Registries must exist at root (https://github.com/docker/docker/issues/7067#issuecomment-54302847)
	// This cannot support the `library/` shortcut for private registries.
	parts := strings.Split(repo, "/")
	switch len(parts) {
	case 1:
		repo = "library/" + repo
	case 2:
		if strings.Contains(repo, ".") {
			registry = parts[0]
			repo = parts[1]
		}
	case 3:
		registry = parts[0]
		repo = parts[1] + "/" + parts[2]
	}

	if tag == "" {
		tag = "latest"
	}

	return registry, repo, tag
}
