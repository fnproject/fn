// Interface for all container drivers

package drivers

import (
	"context"
	"io"
	"strings"

	"github.com/fnproject/fn/api/agent/drivers/stats"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
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

	// Validate/Inspect image. Returns true if the image needs
	// to be pulled and non-nil error if validation/inspection fails.
	ValidateImage(ctx context.Context) (bool, error)

	// Pull the image. An image pull requires validation/inspection
	// again.
	PullImage(ctx context.Context) error

	// Create container which can be Run() later
	CreateContainer(ctx context.Context) error

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

// Check if the provided error is retriable. Returns true and a tag reason if the error is retriable.
type RetryErrorChecker func(error) (bool, string)

type Driver interface {
	// Create a new cookie with defaults and/or settings from container task.
	// Callers should Close the cookie regardless of whether they prepare or run it.
	CreateCookie(ctx context.Context, task ContainerTask) (Cookie, error)

	// Set image pull retry policy and retriable error checker
	SetPullImageRetryPolicy(policy common.BackOffConfig, checker RetryErrorChecker) error

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

	// Driver will write output log from task execution to these writers. Must be
	// non-nil. Use io.Discard if log is irrelevant.
	Logger() (stdout, stderr io.Writer)

	// WriteStat writes a single Stat, implementation need not be thread safe.
	WriteStat(context.Context, stats.Stat)

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

	// AddCloseWrapper is used to add additional cleanup to a task.
	// The original close operation is passed to the wrapping factory.
	// Implementation need not be thread-safe.
	WrapClose(func(closer func()) func())

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

	// Returns true if network is disabled.
	DisableNet() bool

	// BeforeCall is invoked just prior to running an invocation.
	// The Task is definitely going to be used for this invocation.
	// Invocation extensions are passed to the Before and After calls
	BeforeCall(context.Context, *models.Call, CallExtensions) error

	// WrapBeforeCall can add additional pre-call behaviour to be added.
	// This should be called once per ContainerTaskand applies to all successive calls
	// that utilise this task
	WrapBeforeCall(func(BeforeCall) BeforeCall)

	// AfterCall is invoked just after an invocation is finished,
	// providing that BeforeCall returned without an error
	AfterCall(context.Context, *models.Call, CallExtensions) error

	// WrapBeforeCall can add additional post-call behaviour to be added.
	// This should be called once per ContainerTask and applies to all successive calls
	// that utilise this task
	WrapAfterCall(func(AfterCall) AfterCall)
}

type CallExtensions = map[string]string
type BeforeCall = func(context.Context, *models.Call, CallExtensions) error
type AfterCall = func(context.Context, *models.Call, CallExtensions) error

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
	ContainerLabelTag    string `json:"container_label_tag"`
	InstanceId           string `json:"instance_id"`
	ImageCleanMaxSize    uint64 `json:"image_clean_max_size"`
	ImageCleanExemptTags string `json:"image_clean_exempt_tags"`
	ImageEnableVolume    bool   `json:"image_enable_volume"`
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
