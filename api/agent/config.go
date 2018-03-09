package agent

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"time"
)

type AgentConfig struct {
	MinDockerVersion string        `json:"min_docker_version"`
	FreezeIdleMsecs  time.Duration `json:"freeze_idle_msecs"`
	EjectIdleMsecs   time.Duration `json:"eject_idle_msecs"`
	MaxResponseSize  uint64        `json:"max_response_size"`
	MaxLogSize       uint64        `json:"max_log_size"`
}

var MaxDisabledMsecs = time.Duration(math.MaxInt64)

func NewAgentConfig() (*AgentConfig, error) {

	var err error

	cfg := &AgentConfig{
		MinDockerVersion: "17.06.0-ce",
		MaxLogSize:       1 * 1024 * 1024,
	}

	cfg.FreezeIdleMsecs, err = getEnvMsecs("FN_FREEZE_IDLE_MSECS", 50*time.Millisecond)
	if err != nil {
		return cfg, errors.New("error initializing freeze idle delay")
	}

	if tmp := os.Getenv("FN_MAX_LOG_SIZE"); tmp != "" {
		cfg.MaxLogSize, err = strconv.ParseUint(tmp, 10, 64)
		if err != nil {
			return cfg, errors.New("error initializing max log size")
		}
		// for safety during uint64 to int conversions in Write()/Read(), etc.
		if cfg.MaxLogSize > math.MaxInt32 {
			return cfg, fmt.Errorf("error invalid max log size %v > %v", cfg.MaxLogSize, math.MaxInt32)
		}
	}

	cfg.EjectIdleMsecs, err = getEnvMsecs("FN_EJECT_IDLE_MSECS", 1000*time.Millisecond)
	if err != nil {
		return cfg, errors.New("error initializing eject idle delay")
	}

	if cfg.EjectIdleMsecs == time.Duration(0) {
		return cfg, errors.New("error eject idle delay cannot be zero")
	}

	if tmp := os.Getenv("FN_MAX_RESPONSE_SIZE"); tmp != "" {
		cfg.MaxResponseSize, err = strconv.ParseUint(tmp, 10, 64)
		if err != nil {
			return cfg, errors.New("error initializing response buffer size")
		}
	}

	return cfg, nil
}

func getEnvMsecs(name string, defaultVal time.Duration) (time.Duration, error) {

	delay := defaultVal

	if dur := os.Getenv(name); dur != "" {
		durInt, err := strconv.ParseInt(dur, 10, 64)
		if err != nil {
			return defaultVal, err
		}
		// disable if negative or set to msecs specified.
		if durInt < 0 || time.Duration(durInt) >= MaxDisabledMsecs/time.Millisecond {
			delay = MaxDisabledMsecs
		} else {
			delay = time.Duration(durInt) * time.Millisecond
		}
	}

	return delay, nil
}
