package agent

import (
	"errors"
	"math"
	"os"
	"strconv"
	"time"
)

type AgentConfig struct {
	MinDockerVersion string        `json:"minDockerVersion"`
	FreezeIdleMsecs  time.Duration `json:"freezeIdleMsecs"`
	EjectIdleMsecs   time.Duration `json:"ejectIdleMsecs"`
}

func NewAgentConfig() (*AgentConfig, error) {

	var err error

	cfg := &AgentConfig{
		MinDockerVersion: "17.06.0-ce",
	}

	cfg.FreezeIdleMsecs, err = getEnvMsecs("FN_FREEZE_IDLE_MSECS", 50*time.Millisecond)
	if err != nil {
		return cfg, errors.New("error initializing freeze idle delay")
	}

	cfg.EjectIdleMsecs, err = getEnvMsecs("FN_EJECT_IDLE_MSECS", 1000*time.Millisecond)
	if err != nil {
		return cfg, errors.New("error initializing eject idle delay")
	}

	if cfg.EjectIdleMsecs == time.Duration(0) {
		return cfg, errors.New("error eject idle delay cannot be zero")
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
		if durInt < 0 || time.Duration(durInt) >= math.MaxInt64/time.Millisecond {
			delay = math.MaxInt64
		} else {
			delay = time.Duration(durInt) * time.Millisecond
		}
	}

	return delay, nil
}
