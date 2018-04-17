package common

import (
	"time"
)

func MinDuration(f, s time.Duration) time.Duration {
	if f < s {
		return f
	}
	return s
}
