package agent

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"time"
)

type AgentConfig struct {
	MinDockerVersion        string        `json:"min_docker_version"`
	DockerNetworks          string        `json:"docker_networks"`
	FreezeIdle              time.Duration `json:"freeze_idle_msecs"`
	EjectIdle               time.Duration `json:"eject_idle_msecs"`
	HotPoll                 time.Duration `json:"hot_poll_msecs"`
	HotLauncherTimeout      time.Duration `json:"hot_launcher_timeout_msecs"`
	AsyncChewPoll           time.Duration `json:"async_chew_poll_msecs"`
	CallEndTimeout          time.Duration `json:"call_end_timeout"`
	MaxCallEndStacking      uint64        `json:"max_call_end_stacking"`
	MaxResponseSize         uint64        `json:"max_response_size_bytes"`
	MaxRequestSize          uint64        `json:"max_request_size_bytes"`
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
}

const (
	EnvDockerNetworks          = "FN_DOCKER_NETWORKS"
	EnvFreezeIdle              = "FN_FREEZE_IDLE_MSECS"
	EnvEjectIdle               = "FN_EJECT_IDLE_MSECS"
	EnvHotPoll                 = "FN_HOT_POLL_MSECS"
	EnvHotLauncherTimeout      = "FN_HOT_LAUNCHER_TIMEOUT_MSECS"
	EnvAsyncChewPoll           = "FN_ASYNC_CHEW_POLL_MSECS"
	EnvCallEndTimeout          = "FN_CALL_END_TIMEOUT_MSECS"
	EnvMaxCallEndStacking      = "FN_MAX_CALL_END_STACKING"
	EnvMaxResponseSize         = "FN_MAX_RESPONSE_SIZE"
	EnvMaxRequestSize          = "FN_MAX_REQUEST_SIZE"
	EnvMaxLogSize              = "FN_MAX_LOG_SIZE_BYTES"
	EnvMaxTotalCPU             = "FN_MAX_TOTAL_CPU_MCPUS"
	EnvMaxTotalMemory          = "FN_MAX_TOTAL_MEMORY_BYTES"
	EnvMaxFsSize               = "FN_MAX_FS_SIZE_MB"
	EnvPreForkPoolSize         = "FN_EXPERIMENTAL_PREFORK_POOL_SIZE"
	EnvPreForkImage            = "FN_EXPERIMENTAL_PREFORK_IMAGE"
	EnvPreForkCmd              = "FN_EXPERIMENTAL_PREFORK_CMD"
	EnvPreForkUseOnce          = "FN_EXPERIMENTAL_PREFORK_USE_ONCE"
	EnvPreForkNetworks         = "FN_EXPERIMENTAL_PREFORK_NETWORKS"
	EnvEnableNBResourceTracker = "FN_ENABLE_NB_RESOURCE_TRACKER"

	MaxDisabledMsecs = time.Duration(math.MaxInt64)
)

func NewAgentConfig() (*AgentConfig, error) {

	cfg := &AgentConfig{
		MinDockerVersion:   "17.10.0-ce",
		MaxLogSize:         1 * 1024 * 1024,
		MaxCallEndStacking: 8192,
		PreForkImage:       "busybox",
		PreForkCmd:         "tail -f /dev/null",
	}

	var err error

	err = setEnvMsecs(err, EnvFreezeIdle, &cfg.FreezeIdle, 50*time.Millisecond)
	err = setEnvMsecs(err, EnvEjectIdle, &cfg.EjectIdle, 1000*time.Millisecond)
	err = setEnvMsecs(err, EnvHotPoll, &cfg.HotPoll, 200*time.Millisecond)
	err = setEnvMsecs(err, EnvHotLauncherTimeout, &cfg.HotLauncherTimeout, time.Duration(60)*time.Minute)
	err = setEnvMsecs(err, EnvAsyncChewPoll, &cfg.AsyncChewPoll, time.Duration(60)*time.Second)
	err = setEnvMsecs(err, EnvCallEndTimeout, &cfg.CallEndTimeout, time.Duration(10)*time.Minute)
	err = setEnvUint(err, EnvMaxResponseSize, &cfg.MaxResponseSize)
	err = setEnvUint(err, EnvMaxRequestSize, &cfg.MaxRequestSize)
	err = setEnvUint(err, EnvMaxLogSize, &cfg.MaxLogSize)
	err = setEnvUint(err, EnvMaxTotalCPU, &cfg.MaxTotalCPU)
	err = setEnvUint(err, EnvMaxTotalMemory, &cfg.MaxTotalMemory)
	err = setEnvUint(err, EnvMaxFsSize, &cfg.MaxFsSize)
	err = setEnvUint(err, EnvPreForkPoolSize, &cfg.PreForkPoolSize)
	err = setEnvUint(err, EnvMaxCallEndStacking, &cfg.MaxCallEndStacking)
	err = setEnvStr(err, EnvPreForkImage, &cfg.PreForkImage)
	err = setEnvStr(err, EnvPreForkCmd, &cfg.PreForkCmd)
	err = setEnvUint(err, EnvPreForkUseOnce, &cfg.PreForkUseOnce)
	err = setEnvStr(err, EnvPreForkNetworks, &cfg.PreForkNetworks)
	err = setEnvStr(err, EnvDockerNetworks, &cfg.DockerNetworks)

	if err != nil {
		return cfg, err
	}

	if _, ok := os.LookupEnv(EnvEnableNBResourceTracker); ok {
		cfg.EnableNBResourceTracker = true
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
		if durInt < 0 || time.Duration(durInt) >= MaxDisabledMsecs/time.Millisecond {
			*dst = MaxDisabledMsecs
		} else {
			*dst = time.Duration(durInt) * time.Millisecond
		}
	}

	return nil
}
