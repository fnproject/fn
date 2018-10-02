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
}

func NewPlacerConfig(timeout time.Duration) PlacerConfig {
	return PlacerConfig{
		RetryAllDelay: 10 * time.Millisecond,
		PlacerTimeout: timeout * time.Second,
	}
}
