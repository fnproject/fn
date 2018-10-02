package runnerpool

import (
	"time"
)

// Common config for placers.
type PlacerConfig struct {
	// After all runners in the runner list is tried, apply a delay before retrying.
	RetryAllDelay time.Duration `json:"retry_all_delay"`

	// Maximum amount of time a placer can hold a request during runner attempts
	PlacerTimeout time.Duration `json:"placer_timeout"`

	// Maximum amount of time a placer can hold an ack sync request during runner attempts
	DetachedPlacerTimeout time.Duration `json:"detached_placer_timeout"`
}

func NewPlacerConfig() PlacerConfig {
	return PlacerConfig{
		RetryAllDelay:         10 * time.Millisecond,
		PlacerTimeout:         360 * time.Second,
		DetachedPlacerTimeout: 30 * time.Second,
	}
}
