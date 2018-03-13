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
	FreezeIdleMsecs         time.Duration `json:"freeze_idle_msecs"`
	EjectIdleMsecs          time.Duration `json:"eject_idle_msecs"`
	HotPollMsecs            time.Duration `json:"hot_poll_msecs"`
	HotLauncherTimeoutMsecs time.Duration `json:"hot_launcher_timeout_msecs"`
	AsyncChewPollMsecs      time.Duration `json:"async_chew_poll_msecs"`
	MaxResponseSize         uint64        `json:"max_response_size_bytes"`
	MaxLogSize              uint64        `json:"max_log_size_bytes"`
	MaxTotalCPU             uint64        `json:"max_total_cpu_mcpus"`
	MaxTotalMemory          uint64        `json:"max_total_memory_bytes"`
}

const (
	EnvFreezeIdleMsecs         = "FN_FREEZE_IDLE_MSECS"
	EnvEjectIdleMsecs          = "FN_EJECT_IDLE_MSECS"
	EnvHotPollMsecs            = "FN_HOT_POLL_MSECS"
	EnvHotLauncherTimeoutMsecs = "FN_HOT_LAUNCHER_TIMEOUT_MSECS"
	EnvAsyncChewPollMsecs      = "FN_ASYNC_CHEW_POLL_MSECS"
	EnvMaxResponseSize         = "FN_MAX_RESPONSE_SIZE_BYTES"
	EnvMaxLogSize              = "FN_MAX_LOG_SIZE_BYTES"
	EnvMaxTotalCPU             = "FN_MAX_TOTAL_CPU_MCPUS"
	EnvMaxTotalMemory          = "FN_MAX_TOTAL_MEMORY_BYTES"

	MaxDisabledMsecs = time.Duration(math.MaxInt64)
)

func NewAgentConfig() (*AgentConfig, error) {

	cfg := &AgentConfig{
		MinDockerVersion: "17.06.0-ce",
		MaxLogSize:       1 * 1024 * 1024,
	}

	var err error

	err = setEnvMsecs(err, EnvFreezeIdleMsecs, &cfg.FreezeIdleMsecs, 50*time.Millisecond)
	err = setEnvMsecs(err, EnvEjectIdleMsecs, &cfg.EjectIdleMsecs, 1000*time.Millisecond)
	err = setEnvMsecs(err, EnvHotPollMsecs, &cfg.HotPollMsecs, 200*time.Millisecond)
	err = setEnvMsecs(err, EnvHotLauncherTimeoutMsecs, &cfg.HotLauncherTimeoutMsecs, time.Duration(60)*time.Minute)
	err = setEnvMsecs(err, EnvAsyncChewPollMsecs, &cfg.AsyncChewPollMsecs, time.Duration(60)*time.Second)
	err = setEnvUint(err, EnvMaxResponseSize, &cfg.MaxResponseSize)
	err = setEnvUint(err, EnvMaxLogSize, &cfg.MaxLogSize)
	err = setEnvUint(err, EnvMaxTotalCPU, &cfg.MaxTotalCPU)
	err = setEnvUint(err, EnvMaxTotalMemory, &cfg.MaxTotalMemory)

	if err != nil {
		return cfg, err
	}

	if cfg.EjectIdleMsecs == time.Duration(0) {
		return cfg, fmt.Errorf("error %s cannot be zero", EnvEjectIdleMsecs)
	}
	if cfg.MaxLogSize > math.MaxInt32 {
		// for safety during uint64 to int conversions in Write()/Read(), etc.
		return cfg, fmt.Errorf("error invalid %s %v > %v", EnvMaxLogSize, cfg.MaxLogSize, math.MaxInt32)
	}

	return cfg, nil
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
