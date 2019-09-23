package agent

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config specifies various settings for an agent
type Config struct {
	MinDockerVersion              string        `json:"min_docker_version"`
	ContainerLabelTag             string        `json:"container_label_tag"`
	DockerNetworks                string        `json:"docker_networks"`
	DockerLoadFile                string        `json:"docker_load_file"`
	DisableUnprivilegedContainers bool          `json:"disable_unprivileged_containers"`
	FreezeIdle                    time.Duration `json:"freeze_idle_msecs"`
	HotPoll                       time.Duration `json:"hot_poll_msecs"`
	HotLauncherTimeout            time.Duration `json:"hot_launcher_timeout_msecs"`
	HotPullTimeout                time.Duration `json:"hot_pull_timeout_msecs"`
	HotStartTimeout               time.Duration `json:"hot_start_timeout_msecs"`
	DetachedHeadRoom              time.Duration `json:"detached_head_room_msecs"`
	MaxResponseSize               uint64        `json:"max_response_size_bytes"`
	MaxHdrResponseSize            uint64        `json:"max_hdr_response_size_bytes"`
	MaxLogSize                    uint64        `json:"max_log_size_bytes"`
	MaxTotalCPU                   uint64        `json:"max_total_cpu_mcpus"`
	MaxTotalMemory                uint64        `json:"max_total_memory_bytes"`
	MaxFsSize                     uint64        `json:"max_fs_size_mb"`
	MaxPIDs                       uint64        `json:"max_pids"`
	MaxOpenFiles                  *uint64       `json:"max_open_files"`
	MaxLockedMemory               *uint64       `json:"max_locked_memory"`
	MaxPendingSignals             *uint64       `json:"max_pending_signals"`
	MaxMessageQueue               *uint64       `json:"max_message_queue"`
	PreForkPoolSize               uint64        `json:"pre_fork_pool_size"`
	PreForkImage                  string        `json:"pre_fork_image"`
	PreForkCmd                    string        `json:"pre_fork_pool_cmd"`
	PreForkUseOnce                uint64        `json:"pre_fork_use_once"`
	PreForkNetworks               string        `json:"pre_fork_networks"`
	EnableNBResourceTracker       bool          `json:"enable_nb_resource_tracker"`
	MaxTmpFsInodes                uint64        `json:"max_tmpfs_inodes"`
	DisableReadOnlyRootFs         bool          `json:"disable_readonly_rootfs"`
	DisableDebugUserLogs          bool          `json:"disable_debug_user_logs"`
	IOFSEnableTmpfs               bool          `json:"iofs_enable_tmpfs"`
	EnableFDKDebugInfo            bool          `json:"enable_fdk_debug_info"`
	IOFSAgentPath                 string        `json:"iofs_path"`
	IOFSMountRoot                 string        `json:"iofs_mount_root"`
	IOFSOpts                      string        `json:"iofs_opts"`
	ImageCleanMaxSize             uint64        `json:"image_clean_max_size"`
	ImageCleanExemptTags          string        `json:"image_clean_exempt_tags"`
	ImageEnableVolume             bool          `json:"image_enable_volume"`
}

const (
	// EnvContainerLabelTag is a classifier label tag that is used to distinguish fn managed containers
	EnvContainerLabelTag = "FN_CONTAINER_LABEL_TAG"
	// EnvImageCleanMaxSize enables image cleaner and sets the high water mark for image cache in bytes
	EnvImageCleanMaxSize = "FN_IMAGE_CLEAN_MAX_SIZE"
	// EnvImageCleanExemptTags list of image names separated by whitespace that are exempt from removal in image cleaner
	EnvImageCleanExemptTags = "FN_IMAGE_CLEAN_EXEMPT_TAGS"
	// EnvImageEnableVolume allows image to contain VOLUME definitions
	EnvImageEnableVolume = "FN_IMAGE_ENABLE_VOLUME"
	// EnvDockerNetworks is a comma separated list of networks to attach to each container started
	EnvDockerNetworks = "FN_DOCKER_NETWORKS"
	// EnvDockerLoadFile is a file location for a file that contains a tarball of a docker image to load on startup
	EnvDockerLoadFile = "FN_DOCKER_LOAD_FILE"
	// EnvDisableUnprivilegedContainers disables docker security features like user name, cap drop etc.
	EnvDisableUnprivilegedContainers = "FN_DISABLE_UNPRIVILEGED_CONTAINERS"
	// EnvFreezeIdle is the delay between a container being last used and being frozen
	EnvFreezeIdle = "FN_FREEZE_IDLE_MSECS"
	// EnvHotPoll is the interval to ping for a slot manager thread to check if a container should be
	// launched for a given function
	EnvHotPoll = "FN_HOT_POLL_MSECS"
	// EnvHotLauncherTimeout is the timeout for a hot container queue to persist if idle
	EnvHotLauncherTimeout = "FN_HOT_LAUNCHER_TIMEOUT_MSECS"
	// EnvHotStartTimeout is the timeout for a hot container to be created including docker-pull
	EnvHotPullTimeout = "FN_HOT_PULL_TIMEOUT_MSECS"
	// EnvHotStartTimeout is the timeout for a hot container to become available for use for requests after EnvHotStartTimeout
	EnvHotStartTimeout = "FN_HOT_START_TIMEOUT_MSECS"
	// EnvMaxResponseSize is the maximum number of bytes that a function may return from an invocation
	EnvMaxResponseSize = "FN_MAX_RESPONSE_SIZE"
	// EnvHdrMaxResponseSize is the maximum number of bytes that a function may return in an invocation header
	EnvMaxHdrResponseSize = "FN_MAX_HDR_RESPONSE_SIZE"
	// EnvMaxLogSize is the maximum size that a function's log may reach
	EnvMaxLogSize = "FN_MAX_LOG_SIZE_BYTES"
	// EnvMaxTotalCPU is the maximum CPU that will be reserved across all containers
	EnvMaxTotalCPU = "FN_MAX_TOTAL_CPU_MCPUS"
	// EnvMaxTotalMemory is the maximum memory that will be reserved across all containers
	EnvMaxTotalMemory = "FN_MAX_TOTAL_MEMORY_BYTES"
	// EnvMaxFsSize is the maximum filesystem size that a function may use
	EnvMaxFsSize = "FN_MAX_FS_SIZE_MB"
	// EnvMaxPIDs is the maximum number of PIDs that a function is allowed to create
	EnvMaxPIDs = "FN_MAX_PIDS"
	// EnvMaxOpenFiles is the maximum number open files handles the process in a
	// function is allowed to have
	EnvMaxOpenFiles = "FN_MAX_OPEN_FILES"
	// EnvMaxLockedMemory the maximum number of bytes of memory that may be
	// locked into RAM
	EnvMaxLockedMemory = "FN_MAX_LOCKED_MEMORY"
	// EnvMaxPendingSignals limit on the number of signals that may be queued
	EnvMaxPendingSignals = "FN_MAX_PENDING_SIGNALS"
	// EnvMaxMessageQueue limit on the number of bytes that can be allocated for
	// POSIX message queues
	EnvMaxMessageQueue = "FN_MAX_MESSAGE_QUEUE"
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

	// EnvDetachedHeadroom is the extra room we want to give to a detached function to run.
	EnvDetachedHeadroom = "FN_EXECUTION_HEADROOM"

	// MaxMsDisabled is used to determine whether mr freeze is lying in wait. TODO remove this manuever
	MaxMsDisabled = time.Duration(math.MaxInt64)

	// defaults

	// DefaultHotPoll is the default value for EnvHotPoll
	DefaultHotPoll = 200 * time.Millisecond

	// TODO(reed): none of these consts above or below should be exported yo

	// iofsDockerMountDest is the mount path for inside of the container to use for the iofs path
	iofsDockerMountDest = "/tmp/iofs"

	// udsFilename is the file name for the uds socket
	udsFilename = "lsnr.sock"
)

// NewConfig returns a config set from env vars, plus defaults
func NewConfig() (*Config, error) {
	cfg := &Config{
		MinDockerVersion: "17.10.0-ce",
		MaxLogSize:       1 * 1024 * 1024,
		PreForkImage:     "busybox",
		PreForkCmd:       "tail -f /dev/null",
	}

	defaultMaxPIDs := uint64(50)
	defaultMaxOpenFiles := uint64(350)
	defaultMaxLockedMemory := uint64(64 * 1024)
	defaultMaxPendingSignals := uint64(5000)
	defaultMaxMessageQueue := uint64(819200)

	var err error
	err = setEnvMsecs(err, EnvFreezeIdle, &cfg.FreezeIdle, 50*time.Millisecond)
	err = setEnvMsecs(err, EnvHotPoll, &cfg.HotPoll, DefaultHotPoll)
	err = setEnvMsecs(err, EnvHotLauncherTimeout, &cfg.HotLauncherTimeout, time.Duration(60)*time.Minute)
	err = setEnvMsecs(err, EnvHotPullTimeout, &cfg.HotPullTimeout, time.Duration(10)*time.Minute)
	err = setEnvMsecs(err, EnvHotStartTimeout, &cfg.HotStartTimeout, time.Duration(5)*time.Second)
	err = setEnvMsecs(err, EnvDetachedHeadroom, &cfg.DetachedHeadRoom, time.Duration(360)*time.Second)
	err = setEnvUint(err, EnvMaxResponseSize, &cfg.MaxResponseSize, nil)
	err = setEnvUint(err, EnvMaxHdrResponseSize, &cfg.MaxHdrResponseSize, nil)
	err = setEnvUint(err, EnvMaxLogSize, &cfg.MaxLogSize, nil)
	err = setEnvUint(err, EnvMaxTotalCPU, &cfg.MaxTotalCPU, nil)
	err = setEnvUint(err, EnvMaxTotalMemory, &cfg.MaxTotalMemory, nil)
	err = setEnvUint(err, EnvMaxFsSize, &cfg.MaxFsSize, nil)
	err = setEnvUint(err, EnvMaxPIDs, &cfg.MaxPIDs, &defaultMaxPIDs)
	err = setEnvUintPointer(err, EnvMaxOpenFiles, &cfg.MaxOpenFiles, &defaultMaxOpenFiles)
	err = setEnvUintPointer(err, EnvMaxLockedMemory, &cfg.MaxLockedMemory, &defaultMaxLockedMemory)
	err = setEnvUintPointer(err, EnvMaxPendingSignals, &cfg.MaxPendingSignals, &defaultMaxPendingSignals)
	err = setEnvUintPointer(err, EnvMaxMessageQueue, &cfg.MaxMessageQueue, &defaultMaxMessageQueue)
	err = setEnvUint(err, EnvPreForkPoolSize, &cfg.PreForkPoolSize, nil)
	err = setEnvStr(err, EnvPreForkImage, &cfg.PreForkImage)
	err = setEnvStr(err, EnvPreForkCmd, &cfg.PreForkCmd)
	err = setEnvUint(err, EnvPreForkUseOnce, &cfg.PreForkUseOnce, nil)
	err = setEnvStr(err, EnvPreForkNetworks, &cfg.PreForkNetworks)
	err = setEnvStr(err, EnvContainerLabelTag, &cfg.ContainerLabelTag)
	err = setEnvStr(err, EnvDockerNetworks, &cfg.DockerNetworks)
	err = setEnvStr(err, EnvDockerLoadFile, &cfg.DockerLoadFile)
	err = setEnvBool(err, EnvDisableUnprivilegedContainers, &cfg.DisableUnprivilegedContainers)
	err = setEnvUint(err, EnvMaxTmpFsInodes, &cfg.MaxTmpFsInodes, nil)
	err = setEnvStr(err, EnvIOFSPath, &cfg.IOFSAgentPath)
	err = setEnvStr(err, EnvIOFSDockerPath, &cfg.IOFSMountRoot)
	err = setEnvStr(err, EnvIOFSOpts, &cfg.IOFSOpts)
	err = setEnvBool(err, EnvIOFSEnableTmpfs, &cfg.IOFSEnableTmpfs)
	err = setEnvBool(err, EnvEnableNBResourceTracker, &cfg.EnableNBResourceTracker)
	err = setEnvBool(err, EnvDisableReadOnlyRootFs, &cfg.DisableReadOnlyRootFs)
	err = setEnvBool(err, EnvDisableDebugUserLogs, &cfg.DisableDebugUserLogs)
	err = setEnvUint(err, EnvImageCleanMaxSize, &cfg.ImageCleanMaxSize, nil)
	err = setEnvStr(err, EnvImageCleanExemptTags, &cfg.ImageCleanExemptTags)
	err = setEnvBool(err, EnvImageEnableVolume, &cfg.ImageEnableVolume)

	if err != nil {
		return cfg, err
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
		*dst = strings.TrimSpace(tmp)
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

func setEnvUint(err error, name string, dst *uint64, defaultValue *uint64) error {
	if err != nil {
		return err
	}

	if tmp := os.Getenv(name); tmp != "" {
		val, err := strconv.ParseUint(tmp, 10, 64)
		if err != nil {
			return fmt.Errorf("error invalid %s=%s", name, tmp)
		}
		*dst = val
	} else if defaultValue != nil {
		*dst = *defaultValue
	}

	return nil
}

func setEnvUintPointer(err error, name string, dst **uint64, defaultVal *uint64) error {
	if err != nil {
		return err
	}
	if tmp, ok := os.LookupEnv(name); ok {
		val, err := strconv.ParseUint(tmp, 10, 64)
		if err != nil {
			return fmt.Errorf("error invalid %s=%s", name, tmp)
		}
		*dst = &val
	} else if defaultVal != nil {
		// No value found in the environment but a default value is supplied
		*dst = defaultVal
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
