package agent

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"time"
)

// Config specifies various settings for an agent
type Config struct {
	MinDockerVersion        string        `json:"min_docker_version"`
	DockerNetworks          string        `json:"docker_networks"`
	DockerLoadFile          string        `json:"docker_load_file"`
	FreezeIdle              time.Duration `json:"freeze_idle_msecs"`
	EjectIdle               time.Duration `json:"eject_idle_msecs"`
	HotPoll                 time.Duration `json:"hot_poll_msecs"`
	HotLauncherTimeout      time.Duration `json:"hot_launcher_timeout_msecs"`
	AsyncChewPoll           time.Duration `json:"async_chew_poll_msecs"`
	MaxResponseSize         uint64        `json:"max_response_size_bytes"`
	MaxLogSize              uint64        `json:"max_log_size_bytes"`
	MaxTotalCPU             uint64        `json:"max_total_cpu_mcpus"`
	MaxTotalMemory          uint64        `json:"max_total_memory_bytes"`
	MaxFsSize               uint64        `json:"max_fs_size_mb"`
	PreForkPoolSize         uint64        `json:"pre_fork_pool_size"`
	PreForkImage            string        `json:"pre_fork_image"`
	PreForkCmd              string        `json:"pre_fork_pool_cmd"`
	PreForkUseOnce          uint64        `json:"pre_fork_use_once"`
	PreForkNetworks         string        `json:"pre_fork_networks"`
	EnableNBResourceTracker bool          `json:"enable_nb_resource_tracker"`
	MaxTmpFsInodes          uint64        `json:"max_tmpfs_inodes"`
	DisableReadOnlyRootFs   bool          `json:"disable_readonly_rootfs"`
	DisableTini             bool          `json:"disable_tini"`
	DisableDebugUserLogs    bool          `json:"disable_debug_user_logs"`
	IOFSEnableTmpfs         bool          `json:"iofs_enable_tmpfs"`
	IOFSAgentPath           string        `json:"iofs_path"`
	IOFSMountRoot           string        `json:"iofs_mount_root"`
	IOFSOpts                string        `json:"iofs_opts"`
}

const (
	// EnvDockerNetworks is a comma separated list of networks to attach to each container started
	EnvDockerNetworks = "FN_DOCKER_NETWORKS"
	// EnvDockerLoadFile is a file location for a file that contains a tarball of a docker image to load on startup
	EnvDockerLoadFile = "FN_DOCKER_LOAD_FILE"
	// EnvFreezeIdle is the delay between a container being last used and being frozen
	EnvFreezeIdle = "FN_FREEZE_IDLE_MSECS"
	// EnvEjectIdle is the delay before allowing an idle container to be evictable if another container
	// requests the space for itself
	EnvEjectIdle = "FN_EJECT_IDLE_MSECS"
	// EnvHotPoll is the interval to ping for a slot manager thread to check if a container should be
	// launched for a given function
	EnvHotPoll = "FN_HOT_POLL_MSECS"
	// EnvHotLauncherTimeout is the timeout for a hot container to become available for use
	EnvHotLauncherTimeout = "FN_HOT_LAUNCHER_TIMEOUT_MSECS"
	// EnvAsyncChewPoll is the interval to poll the queue that contains async function invocations
	EnvAsyncChewPoll = "FN_ASYNC_CHEW_POLL_MSECS"
	// EnvMaxResponseSize is the maximum number of bytes that a function may return from an invocation
	EnvMaxResponseSize = "FN_MAX_RESPONSE_SIZE"
	// EnvMaxLogSize is the maximum size that a function's log may reach
	EnvMaxLogSize = "FN_MAX_LOG_SIZE_BYTES"
	// EnvMaxTotalCPU is the maximum CPU that will be reserved across all containers
	EnvMaxTotalCPU = "FN_MAX_TOTAL_CPU_MCPUS"
	// EnvMaxTotalMemory is the maximum memory that will be reserved across all containers
	EnvMaxTotalMemory = "FN_MAX_TOTAL_MEMORY_BYTES"
	// EnvMaxFsSize is the maximum filesystem size that a function may use
	EnvMaxFsSize = "FN_MAX_FS_SIZE_MB"
	// EnvPreForkPoolSize is the number of containers pooled to steal network from, this may reduce latency
	EnvPreForkPoolSize = "FN_EXPERIMENTAL_PREFORK_POOL_SIZE"
	// EnvPreForkImage is the image to use for the pre-fork pool
	EnvPreForkImage = "FN_EXPERIMENTAL_PREFORK_IMAGE"
	// EnvPreForkCmd is the command to run for images in the pre-fork pool, it should run for a long time
	EnvPreForkCmd = "FN_EXPERIMENTAL_PREFORK_CMD"
	// EnvPreForkUseOnce limits the number of times a pre-fork pool container may be used to one, they are otherwise recycled
	EnvPreForkUseOnce = "FN_EXPERIMENTAL_PREFORK_USE_ONCE"
	// EnvPreForkNetworks is the equivalent of EnvDockerNetworks but for pre-fork pool containers
	EnvPreForkNetworks = "FN_EXPERIMENTAL_PREFORK_NETWORKS"
	// EnvEnableNBResourceTracker makes every request to the resource tracker non-blocking, meaning the resources are either
	// available or it will return an error immediately
	EnvEnableNBResourceTracker = "FN_ENABLE_NB_RESOURCE_TRACKER"
	// EnvMaxTmpFsInodes is the maximum number of inodes for /tmp in a container
	EnvMaxTmpFsInodes = "FN_MAX_TMPFS_INODES"
	// EnvDisableReadOnlyRootFs makes the root fs for a container have rw permissions, by default it is read only
	EnvDisableReadOnlyRootFs = "FN_DISABLE_READONLY_ROOTFS"
	// EnvDisableTini runs containers without using the --init option, for tini pid 1 action
	EnvDisableTini = "FN_DISABLE_TINI"
	// EnvDisableDebugUserLogs disables user function logs being logged at level debug. wise to enable for production.
	EnvDisableDebugUserLogs = "FN_DISABLE_DEBUG_USER_LOGS"

	// EnvIOFSEnableTmpfs enables creating a per-container tmpfs mount for the IOFS
	EnvIOFSEnableTmpfs = "FN_IOFS_TMPFS"
	// EnvIOFSPath is the path within fn server container of a directory to configure for unix socket files for each container
	EnvIOFSPath = "FN_IOFS_PATH"
	// EnvIOFSDockerPath determines the relative location on the docker host where iofs mounts should be prefixed with
	EnvIOFSDockerPath = "FN_IOFS_DOCKER_PATH"
	// EnvIOFSOpts are the options to set when mounting the iofs directory for unix socket files
	EnvIOFSOpts = "FN_IOFS_OPTS"

	// MaxMsDisabled is used to determine whether mr freeze is lying in wait. TODO remove this manuever
	MaxMsDisabled = time.Duration(math.MaxInt64)

	// defaults

	// DefaultHotPoll is the default value for EnvHotPoll
	DefaultHotPoll = 200 * time.Millisecond
)

// NewConfig returns a config set from env vars, plus defaults
func NewConfig() (*Config, error) {

	cfg := &Config{
		MinDockerVersion: "17.10.0-ce",
		MaxLogSize:       1 * 1024 * 1024,
		PreForkImage:     "busybox",
		PreForkCmd:       "tail -f /dev/null",
	}

	var err error

	err = setEnvMsecs(err, EnvFreezeIdle, &cfg.FreezeIdle, 50*time.Millisecond)
	err = setEnvMsecs(err, EnvEjectIdle, &cfg.EjectIdle, 1000*time.Millisecond)
	err = setEnvMsecs(err, EnvHotPoll, &cfg.HotPoll, DefaultHotPoll)
	err = setEnvMsecs(err, EnvHotLauncherTimeout, &cfg.HotLauncherTimeout, time.Duration(60)*time.Minute)
	err = setEnvMsecs(err, EnvAsyncChewPoll, &cfg.AsyncChewPoll, time.Duration(60)*time.Second)
	err = setEnvUint(err, EnvMaxResponseSize, &cfg.MaxResponseSize)
	err = setEnvUint(err, EnvMaxLogSize, &cfg.MaxLogSize)
	err = setEnvUint(err, EnvMaxTotalCPU, &cfg.MaxTotalCPU)
	err = setEnvUint(err, EnvMaxTotalMemory, &cfg.MaxTotalMemory)
	err = setEnvUint(err, EnvMaxFsSize, &cfg.MaxFsSize)
	err = setEnvUint(err, EnvPreForkPoolSize, &cfg.PreForkPoolSize)
	err = setEnvStr(err, EnvPreForkImage, &cfg.PreForkImage)
	err = setEnvStr(err, EnvPreForkCmd, &cfg.PreForkCmd)
	err = setEnvUint(err, EnvPreForkUseOnce, &cfg.PreForkUseOnce)
	err = setEnvStr(err, EnvPreForkNetworks, &cfg.PreForkNetworks)
	err = setEnvStr(err, EnvDockerNetworks, &cfg.DockerNetworks)
	err = setEnvStr(err, EnvDockerLoadFile, &cfg.DockerLoadFile)
	err = setEnvUint(err, EnvMaxTmpFsInodes, &cfg.MaxTmpFsInodes)
	err = setEnvStr(err, EnvIOFSPath, &cfg.IOFSAgentPath)
	err = setEnvStr(err, EnvIOFSDockerPath, &cfg.IOFSMountRoot)
	err = setEnvStr(err, EnvIOFSOpts, &cfg.IOFSOpts)
	err = setEnvBool(err, EnvIOFSEnableTmpfs, &cfg.IOFSEnableTmpfs)
	err = setEnvBool(err, EnvEnableNBResourceTracker, &cfg.EnableNBResourceTracker)
	err = setEnvBool(err, EnvDisableReadOnlyRootFs, &cfg.DisableReadOnlyRootFs)
	err = setEnvBool(err, EnvDisableDebugUserLogs, &cfg.DisableDebugUserLogs)

	if err != nil {
		return cfg, err
	}

	if cfg.EjectIdle == time.Duration(0) {
		return cfg, fmt.Errorf("error %s cannot be zero", EnvEjectIdle)
	}
	if cfg.MaxLogSize > math.MaxInt64 {
		// for safety during uint64 to int conversions in Write()/Read(), etc.
		return cfg, fmt.Errorf("error invalid %s %v > %v", EnvMaxLogSize, cfg.MaxLogSize, math.MaxInt64)
	}

	return cfg, nil
}

func setEnvStr(err error, name string, dst *string) error {
	if err != nil {
		return err
	}
	if tmp, ok := os.LookupEnv(name); ok {
		*dst = tmp
	}
	return nil
}

func setEnvBool(err error, name string, dst *bool) error {
	if err != nil {
		return err
	}
	if tmp, ok := os.LookupEnv(name); ok {
		val, err := strconv.ParseBool(tmp)
		if err != nil {
			return err
		}
		*dst = val
	}
	return nil
}

func setEnvUint(err error, name string, dst *uint64) error {
	if err != nil {
		return err
	}
	if tmp := os.Getenv(name); tmp != "" {
		val, err := strconv.ParseUint(tmp, 10, 64)
		if err != nil {
			return fmt.Errorf("error invalid %s=%s", name, tmp)
		}
		*dst = val
	}
	return nil
}

func setEnvMsecs(err error, name string, dst *time.Duration, defaultVal time.Duration) error {
	if err != nil {
		return err
	}

	*dst = defaultVal

	if dur := os.Getenv(name); dur != "" {
		durInt, err := strconv.ParseInt(dur, 10, 64)
		if err != nil {
			return fmt.Errorf("error invalid %s=%s err=%s", name, dur, err)
		}
		// disable if negative or set to msecs specified.
		if durInt < 0 || time.Duration(durInt) >= MaxMsDisabled/time.Millisecond {
			*dst = MaxMsDisabled
		} else {
			*dst = time.Duration(durInt) * time.Millisecond
		}
	}

	return nil
}
